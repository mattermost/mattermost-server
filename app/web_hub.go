// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"hash/maphash"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	broadcastQueueSize         = 4096
	inactiveConnReaperInterval = 5 * time.Minute
)

type webConnActivityMessage struct {
	userID       string
	sessionToken string
	activityAt   int64
}

type webConnDirectMessage struct {
	conn *WebConn
	msg  model.WebSocketMessage
}

type webConnSessionMessage struct {
	userID       string
	sessionToken string
	isRegistered chan bool
}

type webConnCheckMessage struct {
	userID       string
	connectionID string
	result       chan *CheckConnResult
}

// Hub is the central place to manage all websocket connections in the server.
// It handles different websocket events and sending messages to individual
// user connections.
type Hub struct {
	// connectionCount should be kept first.
	// See https://github.com/mattermost/mattermost-server/pull/7281
	connectionCount int64
	app             *App
	connectionIndex int
	register        chan *WebConn
	unregister      chan *WebConn
	broadcast       chan *model.WebSocketEvent
	stop            chan struct{}
	didStop         chan struct{}
	invalidateUser  chan string
	activity        chan *webConnActivityMessage
	directMsg       chan *webConnDirectMessage
	explicitStop    bool
	checkRegistered chan *webConnSessionMessage
	checkConn       chan *webConnCheckMessage
}

// NewWebHub creates a new Hub.
func (a *App) NewWebHub() *Hub {
	return &Hub{
		app:             a,
		register:        make(chan *WebConn),
		unregister:      make(chan *WebConn),
		broadcast:       make(chan *model.WebSocketEvent, broadcastQueueSize),
		stop:            make(chan struct{}),
		didStop:         make(chan struct{}),
		invalidateUser:  make(chan string),
		activity:        make(chan *webConnActivityMessage),
		directMsg:       make(chan *webConnDirectMessage),
		checkRegistered: make(chan *webConnSessionMessage),
		checkConn:       make(chan *webConnCheckMessage),
	}
}

func (a *App) TotalWebsocketConnections() int {
	return a.Srv().TotalWebsocketConnections()
}

// HubStart starts all the hubs.
func (a *App) HubStart() {
	// Total number of hubs is twice the number of CPUs.
	numberOfHubs := runtime.NumCPU() * 2
	mlog.Info("Starting websocket hubs", mlog.Int("number_of_hubs", numberOfHubs))

	hubs := make([]*Hub, numberOfHubs)

	for i := 0; i < numberOfHubs; i++ {
		hubs[i] = a.NewWebHub()
		hubs[i].connectionIndex = i
		hubs[i].Start()
	}
	// Assigning to the hubs slice without any mutex is fine because it is only assigned once
	// during the start of the program and always read from after that.
	a.srv.hubs = hubs
}

func (a *App) invalidateCacheForWebhook(webhookID string) {
	a.Srv().Store.Webhook().InvalidateWebhookCache(webhookID)
}

// HubStop stops all the hubs.
func (s *Server) HubStop() {
	mlog.Info("stopping websocket hub connections")

	for _, hub := range s.hubs {
		hub.Stop()
	}
}

func (a *App) HubStop() {
	a.Srv().HubStop()
}

// GetHubForUserId returns the hub for a given user id.
func (s *Server) GetHubForUserID(userID string) *Hub {
	// TODO: check if caching the userID -> hub mapping
	// is worth the memory tradeoff.
	// https://mattermost.atlassian.net/browse/MM-26629.
	var hash maphash.Hash
	hash.SetSeed(s.hashSeed)
	hash.Write([]byte(userID))
	index := hash.Sum64() % uint64(len(s.hubs))

	return s.hubs[int(index)]
}

func (a *App) GetHubForUserID(userID string) *Hub {
	return a.Srv().GetHubForUserID(userID)
}

// HubRegister registers a connection to a hub.
func (a *App) HubRegister(webConn *WebConn) {
	hub := a.GetHubForUserID(webConn.UserID)
	if hub != nil {
		if metrics := a.Metrics(); metrics != nil {
			metrics.IncrementWebSocketBroadcastUsersRegistered(strconv.Itoa(hub.connectionIndex), 1)
		}
		hub.Register(webConn)
	}
}

// HubUnregister unregisters a connection from a hub.
func (a *App) HubUnregister(webConn *WebConn) {
	hub := a.GetHubForUserID(webConn.UserID)
	if hub != nil {
		if metrics := a.Metrics(); metrics != nil {
			metrics.DecrementWebSocketBroadcastUsersRegistered(strconv.Itoa(hub.connectionIndex), 1)
		}
		hub.Unregister(webConn)
	}
}

func (s *Server) Publish(message *model.WebSocketEvent) {
	if s.Metrics != nil {
		s.Metrics.IncrementWebsocketEvent(message.EventType())
	}

	s.PublishSkipClusterSend(message)

	if s.Cluster != nil {
		cm := &model.ClusterMessage{
			Event:    model.ClusterEventPublish,
			SendType: model.ClusterSendBestEffort,
			Data:     message.ToJSON(),
		}

		if message.EventType() == model.WebsocketEventPosted ||
			message.EventType() == model.WebsocketEventPostEdited ||
			message.EventType() == model.WebsocketEventDirectAdded ||
			message.EventType() == model.WebsocketEventGroupAdded ||
			message.EventType() == model.WebsocketEventAddedToTeam {
			cm.SendType = model.ClusterSendReliable
		}

		s.Cluster.SendClusterMessage(cm)
	}
}

func (a *App) Publish(message *model.WebSocketEvent) {
	a.Srv().Publish(message)
}

func (s *Server) PublishSkipClusterSend(event *model.WebSocketEvent) {
	if event.GetBroadcast().UserID != "" {
		hub := s.GetHubForUserID(event.GetBroadcast().UserID)
		if hub != nil {
			hub.Broadcast(event)
		}
	} else {
		for _, hub := range s.hubs {
			hub.Broadcast(event)
		}
	}

	// Notify shared channel sync service
	s.SharedChannelSyncHandler(event)
}

func (a *App) invalidateCacheForChannel(channel *model.Channel) {
	a.Srv().Store.Channel().InvalidateChannel(channel.ID)
	a.Srv().invalidateCacheForChannelByNameSkipClusterSend(channel.TeamID, channel.Name)

	if a.Cluster() != nil {
		nameMsg := &model.ClusterMessage{
			Event:    model.ClusterEventInvalidateCacheForChannelByName,
			SendType: model.ClusterSendBestEffort,
			Props:    make(map[string]string),
		}

		nameMsg.Props["name"] = channel.Name
		if channel.TeamID == "" {
			nameMsg.Props["id"] = "dm"
		} else {
			nameMsg.Props["id"] = channel.TeamID
		}

		a.Cluster().SendClusterMessage(nameMsg)
	}
}

func (a *App) invalidateCacheForChannelMembers(channelID string) {
	a.Srv().Store.User().InvalidateProfilesInChannelCache(channelID)
	a.Srv().Store.Channel().InvalidateMemberCount(channelID)
	a.Srv().Store.Channel().InvalidateGuestCount(channelID)
}

func (a *App) invalidateCacheForChannelMembersNotifyProps(channelID string) {
	a.Srv().invalidateCacheForChannelMembersNotifyPropsSkipClusterSend(channelID)

	if a.Cluster() != nil {
		msg := &model.ClusterMessage{
			Event:    model.ClusterEventInvalidateCacheForChannelMembersNotifyProps,
			SendType: model.ClusterSendBestEffort,
			Data:     channelID,
		}
		a.Cluster().SendClusterMessage(msg)
	}
}

func (a *App) invalidateCacheForChannelPosts(channelID string) {
	a.Srv().Store.Channel().InvalidatePinnedPostCount(channelID)
	a.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)
}

func (a *App) InvalidateCacheForUser(userID string) {
	a.Srv().invalidateCacheForUserSkipClusterSend(userID)

	a.srv.userService.InvalidateCacheForUser(userID)
}

func (a *App) invalidateCacheForUserTeams(userID string) {
	a.Srv().invalidateWebConnSessionCacheForUser(userID)
	a.Srv().Store.Team().InvalidateAllTeamIDsForUser(userID)

	if a.Cluster() != nil {
		msg := &model.ClusterMessage{
			Event:    model.ClusterEventInvalidateCacheForUserTeams,
			SendType: model.ClusterSendBestEffort,
			Data:     userID,
		}
		a.Cluster().SendClusterMessage(msg)
	}
}

// UpdateWebConnUserActivity sets the LastUserActivityAt of the hub for the given session.
func (a *App) UpdateWebConnUserActivity(session model.Session, activityAt int64) {
	hub := a.GetHubForUserID(session.UserID)
	if hub != nil {
		hub.UpdateActivity(session.UserID, session.Token, activityAt)
	}
}

// SessionIsRegistered determines if a specific session has been registered
func (a *App) SessionIsRegistered(session model.Session) bool {
	hub := a.GetHubForUserID(session.UserID)
	if hub != nil {
		return hub.IsRegistered(session.UserID, session.Token)
	}
	return false
}

func (a *App) CheckWebConn(userID, connectionID string) *CheckConnResult {
	hub := a.GetHubForUserID(userID)
	if hub != nil {
		return hub.CheckConn(userID, connectionID)
	}
	return nil
}

// Register registers a connection to the hub.
func (h *Hub) Register(webConn *WebConn) {
	select {
	case h.register <- webConn:
	case <-h.stop:
	}
}

// Unregister unregisters a connection from the hub.
func (h *Hub) Unregister(webConn *WebConn) {
	select {
	case h.unregister <- webConn:
	case <-h.stop:
	}
}

// Determines if a user's session is registered a connection from the hub.
func (h *Hub) IsRegistered(userID, sessionToken string) bool {
	ws := &webConnSessionMessage{
		userID:       userID,
		sessionToken: sessionToken,
		isRegistered: make(chan bool),
	}
	select {
	case h.checkRegistered <- ws:
		return <-ws.isRegistered
	case <-h.stop:
	}
	return false
}

func (h *Hub) CheckConn(userID, connectionID string) *CheckConnResult {
	req := &webConnCheckMessage{
		userID:       userID,
		connectionID: connectionID,
		result:       make(chan *CheckConnResult),
	}
	select {
	case h.checkConn <- req:
		return <-req.result
	case <-h.stop:
	}
	return nil
}

// Broadcast broadcasts the message to all connections in the hub.
func (h *Hub) Broadcast(message *model.WebSocketEvent) {
	// XXX: The hub nil check is because of the way we setup our tests. We call
	// `app.NewServer()` which returns a server, but only after that, we call
	// `wsapi.Init()` to initialize the hub.  But in the `NewServer` call
	// itself proceeds to broadcast some messages happily.  This needs to be
	// fixed once the wsapi cyclic dependency with server/app goes away.
	// And possibly, we can look into doing the hub initialization inside
	// NewServer itself.
	if h != nil && message != nil {
		if metrics := h.app.Metrics(); metrics != nil {
			metrics.IncrementWebSocketBroadcastBufferSize(strconv.Itoa(h.connectionIndex), 1)
		}
		select {
		case h.broadcast <- message:
		case <-h.stop:
		}
	}
}

// InvalidateUser invalidates the cache for the given user.
func (h *Hub) InvalidateUser(userID string) {
	select {
	case h.invalidateUser <- userID:
	case <-h.stop:
	}
}

// UpdateActivity sets the LastUserActivityAt field for the connection
// of the user.
func (h *Hub) UpdateActivity(userID, sessionToken string, activityAt int64) {
	select {
	case h.activity <- &webConnActivityMessage{
		userID:       userID,
		sessionToken: sessionToken,
		activityAt:   activityAt,
	}:
	case <-h.stop:
	}
}

// SendMessage sends the given message to the given connection.
func (h *Hub) SendMessage(conn *WebConn, msg model.WebSocketMessage) {
	select {
	case h.directMsg <- &webConnDirectMessage{
		conn: conn,
		msg:  msg,
	}:
	case <-h.stop:
	}
}

// Stop stops the hub.
func (h *Hub) Stop() {
	close(h.stop)
	<-h.didStop
}

// Start starts the hub.
func (h *Hub) Start() {
	var doStart func()
	var doRecoverableStart func()
	var doRecover func()

	doStart = func() {
		mlog.Debug("Hub is starting", mlog.Int("index", h.connectionIndex))

		ticker := time.NewTicker(inactiveConnReaperInterval)
		defer ticker.Stop()

		connIndex := newHubConnectionIndex(inactiveConnReaperInterval)

		for {
			select {
			case webSessionMessage := <-h.checkRegistered:
				conns := connIndex.ForUser(webSessionMessage.userID)
				var isRegistered bool
				for _, conn := range conns {
					if !conn.active {
						continue
					}
					if conn.GetSessionToken() == webSessionMessage.sessionToken {
						isRegistered = true
					}
				}
				webSessionMessage.isRegistered <- isRegistered
			case req := <-h.checkConn:
				var res *CheckConnResult
				conn := connIndex.GetInactiveByConnectionID(req.userID, req.connectionID)
				if conn != nil {
					res = &CheckConnResult{
						ConnectionID:     req.connectionID,
						UserID:           req.userID,
						ActiveQueue:      conn.send,
						DeadQueue:        conn.deadQueue,
						DeadQueuePointer: conn.deadQueuePointer,
					}
				}
				req.result <- res
			case <-ticker.C:
				connIndex.RemoveInactiveConnections()
			case webConn := <-h.register:
				var oldConn *WebConn
				if *h.app.Config().ServiceSettings.EnableReliableWebSockets {
					// Delete the old conn from connIndex if it exists.
					oldConn = connIndex.RemoveInactiveByConnectionID(
						webConn.GetSession().UserID,
						webConn.GetConnectionID())
				}

				// Mark the current one as active.
				// There is no need to check if it was inactive or not,
				// we will anyways need to make it active.
				webConn.active = true

				connIndex.Add(webConn)
				atomic.StoreInt64(&h.connectionCount, int64(connIndex.AllActive()))

				if webConn.IsAuthenticated() && oldConn == nil {
					// The hello message should only be sent when the conn wasn't found.
					// i.e in server restart, or long timeout, or fresh connection case.
					// In case of seq number not found in dead queue, it is handled by
					// the webconn write pump.
					webConn.send <- webConn.createHelloMessage()
				}
			case webConn := <-h.unregister:
				// If already removed (via queue full), then removing again becomes a noop.
				// But if not removed, mark inactive.
				if *h.app.Config().ServiceSettings.EnableReliableWebSockets {
					webConn.active = false
				} else {
					connIndex.Remove(webConn)
				}

				atomic.StoreInt64(&h.connectionCount, int64(connIndex.AllActive()))

				if webConn.UserID == "" {
					continue
				}

				conns := connIndex.ForUser(webConn.UserID)
				if len(conns) == 0 || areAllInactive(conns) {
					h.app.Srv().Go(func() {
						h.app.SetStatusOffline(webConn.UserID, false)
					})
					continue
				}
				var latestActivity int64 = 0
				for _, conn := range conns {
					if !conn.active {
						continue
					}
					if conn.lastUserActivityAt > latestActivity {
						latestActivity = conn.lastUserActivityAt
					}
				}

				if h.app.IsUserAway(latestActivity) {
					h.app.Srv().Go(func() {
						h.app.SetStatusLastActivityAt(webConn.UserID, latestActivity)
					})
				}
			case userID := <-h.invalidateUser:
				for _, webConn := range connIndex.ForUser(userID) {
					webConn.InvalidateCache()
				}
			case activity := <-h.activity:
				for _, webConn := range connIndex.ForUser(activity.userID) {
					if !webConn.active {
						continue
					}
					if webConn.GetSessionToken() == activity.sessionToken {
						webConn.lastUserActivityAt = activity.activityAt
					}
				}
			case directMsg := <-h.directMsg:
				if !connIndex.Has(directMsg.conn) {
					continue
				}
				select {
				case directMsg.conn.send <- directMsg.msg:
				default:
					mlog.Error("webhub.broadcast: cannot send, closing websocket for user", mlog.String("user_id", directMsg.conn.UserID))
					close(directMsg.conn.send)
					connIndex.Remove(directMsg.conn)
				}
			case msg := <-h.broadcast:
				if metrics := h.app.Metrics(); metrics != nil {
					metrics.DecrementWebSocketBroadcastBufferSize(strconv.Itoa(h.connectionIndex), 1)
				}
				msg = msg.PrecomputeJSON()
				broadcast := func(webConn *WebConn) {
					if !connIndex.Has(webConn) {
						return
					}
					if webConn.shouldSendEvent(msg) {
						select {
						case webConn.send <- msg:
						default:
							mlog.Error("webhub.broadcast: cannot send, closing websocket for user", mlog.String("user_id", webConn.UserID))
							close(webConn.send)
							connIndex.Remove(webConn)
						}
					}
				}
				if msg.GetBroadcast().UserID != "" {
					candidates := connIndex.ForUser(msg.GetBroadcast().UserID)
					for _, webConn := range candidates {
						broadcast(webConn)
					}
					continue
				}
				candidates := connIndex.All()
				for webConn := range candidates {
					broadcast(webConn)
				}
			case <-h.stop:
				for webConn := range connIndex.All() {
					webConn.Close()
					h.app.SetStatusOffline(webConn.UserID, false)
				}

				h.explicitStop = true
				close(h.didStop)

				return
			}
		}
	}

	doRecoverableStart = func() {
		defer doRecover()
		doStart()
	}

	doRecover = func() {
		if !h.explicitStop {
			if r := recover(); r != nil {
				mlog.Error("Recovering from Hub panic.", mlog.Any("panic", r))
			} else {
				mlog.Error("Webhub stopped unexpectedly. Recovering.")
			}

			mlog.Error(string(debug.Stack()))

			go doRecoverableStart()
		}
	}

	go doRecoverableStart()
}

// hubConnectionIndex provides fast addition, removal, and iteration of web connections.
// It requires 3 functionalities which need to be very fast:
// - check if a connection exists or not.
// - get all connections for a given userID.
// - get all connections.
type hubConnectionIndex struct {
	// byUserId stores the list of connections for a given userID
	byUserID map[string][]*WebConn
	// byConnection serves the dual purpose of storing the index of the webconn
	// in the value of byUserId map, and also to get all connections.
	byConnection map[*WebConn]int
	// staleThreshold is the limit beyond which inactive connections
	// will be deleted.
	staleThreshold time.Duration
}

func newHubConnectionIndex(interval time.Duration) *hubConnectionIndex {
	return &hubConnectionIndex{
		byUserID:       make(map[string][]*WebConn),
		byConnection:   make(map[*WebConn]int),
		staleThreshold: interval,
	}
}

func (i *hubConnectionIndex) Add(wc *WebConn) {
	i.byUserID[wc.UserID] = append(i.byUserID[wc.UserID], wc)
	i.byConnection[wc] = len(i.byUserID[wc.UserID]) - 1
}

func (i *hubConnectionIndex) Remove(wc *WebConn) {
	userConnIndex, ok := i.byConnection[wc]
	if !ok {
		return
	}

	// get the conn slice.
	userConnections := i.byUserID[wc.UserID]
	// get the last connection.
	last := userConnections[len(userConnections)-1]
	// set the slot that we are trying to remove to be the last connection.
	userConnections[userConnIndex] = last
	// remove the last connection from the slice.
	i.byUserID[wc.UserID] = userConnections[:len(userConnections)-1]
	// set the index of the connection that was moved to the new index.
	i.byConnection[last] = userConnIndex

	delete(i.byConnection, wc)
}

func (i *hubConnectionIndex) Has(wc *WebConn) bool {
	_, ok := i.byConnection[wc]
	return ok
}

// ForUser returns all connections for a user ID.
func (i *hubConnectionIndex) ForUser(id string) []*WebConn {
	return i.byUserID[id]
}

// All returns the full webConn index.
func (i *hubConnectionIndex) All() map[*WebConn]int {
	return i.byConnection
}

// GetInactiveByConnectionID returns an inactive connection for the given
// userID and connectionID.
func (i *hubConnectionIndex) GetInactiveByConnectionID(userID, connectionID string) *WebConn {
	// To handle empty sessions.
	if userID == "" {
		return nil
	}
	for _, conn := range i.ForUser(userID) {
		if conn.GetConnectionID() == connectionID && !conn.active {
			return conn
		}
	}
	return nil
}

// RemoveInactiveByConnectionID removes an inactive connection for the given
// userID and connectionID.
func (i *hubConnectionIndex) RemoveInactiveByConnectionID(userID, connectionID string) *WebConn {
	// To handle empty sessions.
	if userID == "" {
		return nil
	}
	for _, conn := range i.ForUser(userID) {
		if conn.GetConnectionID() == connectionID && !conn.active {
			i.Remove(conn)
			return conn
		}
	}
	return nil
}

// RemoveInactiveConnections removes all inactive connections whose lastUserActivityAt
// exceeded staleThreshold.
func (i *hubConnectionIndex) RemoveInactiveConnections() {
	now := model.GetMillis()
	for conn := range i.byConnection {
		if !conn.active && now-conn.lastUserActivityAt > i.staleThreshold.Milliseconds() {
			i.Remove(conn)
		}
	}
}

// AllActive returns the number of active connections.
// This is only called during register/unregister so we can take
// a bit of perf hit here.
func (i *hubConnectionIndex) AllActive() int {
	cnt := 0
	for conn := range i.byConnection {
		if conn.active {
			cnt++
		}
	}
	return cnt
}
