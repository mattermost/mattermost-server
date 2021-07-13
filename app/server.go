// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"golang.org/x/crypto/acme/autocert"

	"github.com/mattermost/mattermost-server/v5/app/featureflag"
	"github.com/mattermost/mattermost-server/v5/app/imaging"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/jobs"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/services/awsmeter"
	"github.com/mattermost/mattermost-server/v5/services/cache"
	"github.com/mattermost/mattermost-server/v5/services/httpservice"
	"github.com/mattermost/mattermost-server/v5/services/imageproxy"
	"github.com/mattermost/mattermost-server/v5/services/remotecluster"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/services/searchengine/bleveengine"
	"github.com/mattermost/mattermost-server/v5/services/sharedchannel"
	"github.com/mattermost/mattermost-server/v5/services/telemetry"
	"github.com/mattermost/mattermost-server/v5/services/timezones"
	"github.com/mattermost/mattermost-server/v5/services/tracing"
	"github.com/mattermost/mattermost-server/v5/services/upgrader"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/filestore"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/shared/templates"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/localcachelayer"
	"github.com/mattermost/mattermost-server/v5/store/retrylayer"
	"github.com/mattermost/mattermost-server/v5/store/searchlayer"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
	"github.com/mattermost/mattermost-server/v5/store/timerlayer"
	"github.com/mattermost/mattermost-server/v5/utils"
)

var MaxNotificationsPerChannelDefault int64 = 1000000

// declaring this as var to allow overriding in tests
var SentryDSN = "placeholder_sentry_dsn"

type Server struct {
	sqlStore        *sqlstore.SqlStore
	Store           store.Store
	WebSocketRouter *WebSocketRouter

	// RootRouter is the starting point for all HTTP requests to the server.
	RootRouter *mux.Router

	// LocalRouter is the starting point for all the local UNIX socket
	// requests to the server
	LocalRouter *mux.Router

	// Router is the starting point for all web, api4 and ws requests to the server. It differs
	// from RootRouter only if the SiteURL contains a /subpath.
	Router *mux.Router

	Server      *http.Server
	ListenAddr  *net.TCPAddr
	RateLimiter *RateLimiter
	Busy        *Busy

	localModeServer *http.Server

	metricsServer *http.Server
	metricsRouter *mux.Router
	metricsLock   sync.Mutex

	didFinishListen chan struct{}

	goroutineCount      int32
	goroutineExitSignal chan struct{}

	PluginsEnvironment     *plugin.Environment
	PluginConfigListenerId string
	PluginsLock            sync.RWMutex

	EmailService *EmailService

	hubs     []*Hub
	hashSeed maphash.Seed

	PushNotificationsHub   PushNotificationsHub
	pushNotificationClient *http.Client // TODO: move this to it's own package

	runEssentialJobs bool
	Jobs             *jobs.JobServer

	clusterLeaderListeners sync.Map

	licenseValue       atomic.Value
	clientLicenseValue atomic.Value
	licenseListeners   map[string]func(*model.License, *model.License)

	timezones *timezones.Timezones

	newStore func() (store.Store, error)

	htmlTemplateWatcher     *templates.Container
	seenPendingPostIdsCache cache.Cache
	statusCache             cache.Cache
	configListenerId        string
	licenseListenerId       string
	logListenerId           string
	clusterLeaderListenerId string
	searchConfigListenerId  string
	searchLicenseListenerId string
	loggerLicenseListenerId string
	configStore             *config.Store
	postActionCookieSecret  []byte

	advancedLogListenerCleanup func()

	pluginCommands     []*PluginCommand
	pluginCommandsLock sync.RWMutex

	asymmetricSigningKey atomic.Value
	clientConfig         atomic.Value
	clientConfigHash     atomic.Value
	limitedClientConfig  atomic.Value

	telemetryService *telemetry.TelemetryService
	userService      *users.UserService

	serviceMux           sync.RWMutex
	remoteClusterService remotecluster.RemoteClusterServiceIFace
	sharedChannelService SharedChannelServiceIFace

	phase2PermissionsMigrationComplete bool

	HTTPService httpservice.HTTPService

	ImageProxy *imageproxy.ImageProxy

	Audit            *audit.Audit
	Log              *mlog.Logger
	NotificationsLog *mlog.Logger

	joinCluster       bool
	startMetrics      bool
	startSearchEngine bool
	skipPostInit      bool

	SearchEngine *searchengine.Broker

	AccountMigration einterfaces.AccountMigrationInterface
	Cluster          einterfaces.ClusterInterface
	Compliance       einterfaces.ComplianceInterface
	DataRetention    einterfaces.DataRetentionInterface
	Ldap             einterfaces.LdapInterface
	MessageExport    einterfaces.MessageExportInterface
	Cloud            einterfaces.CloudInterface
	Metrics          einterfaces.MetricsInterface
	Notification     einterfaces.NotificationInterface
	Saml             einterfaces.SamlInterface
	LicenseManager   einterfaces.LicenseInterface

	CacheProvider cache.Provider

	tracer *tracing.Tracer

	// These are used to prevent concurrent upload requests
	// for a given upload session which could cause inconsistencies
	// and data corruption.
	uploadLockMapMut sync.Mutex
	uploadLockMap    map[string]bool

	featureFlagSynchronizer      *featureflag.Synchronizer
	featureFlagStop              chan struct{}
	featureFlagStopped           chan struct{}
	featureFlagSynchronizerMutex sync.Mutex

	imgDecoder *imaging.Decoder
	imgEncoder *imaging.Encoder

	dndTaskMut sync.Mutex
	dndTask    *model.ScheduledTask
}

func NewServer(options ...Option) (*Server, error) {
	rootRouter := mux.NewRouter()
	localRouter := mux.NewRouter()

	s := &Server{
		goroutineExitSignal: make(chan struct{}, 1),
		RootRouter:          rootRouter,
		LocalRouter:         localRouter,
		licenseListeners:    map[string]func(*model.License, *model.License){},
		hashSeed:            maphash.MakeSeed(),
		uploadLockMap:       map[string]bool{},
	}

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, errors.Wrap(err, "failed to apply option")
		}
	}

	if s.configStore == nil {
		innerStore, err := config.NewFileStore("config.json")
		if err != nil {
			return nil, errors.Wrap(err, "failed to load config")
		}
		configStore, err := config.NewStoreFromBacking(innerStore, nil, false)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load config")
		}

		s.configStore = configStore
	}

	if err := s.initLogging(); err != nil {
		mlog.Error("Could not initiate logging", mlog.Err(err))
	}

	var imgErr error
	s.imgDecoder, imgErr = imaging.NewDecoder(imaging.DecoderOptions{
		ConcurrencyLevel: runtime.NumCPU(),
	})
	if imgErr != nil {
		return nil, errors.Wrap(imgErr, "failed to create image decoder")
	}
	s.imgEncoder, imgErr = imaging.NewEncoder(imaging.EncoderOptions{
		ConcurrencyLevel: runtime.NumCPU(),
	})
	if imgErr != nil {
		return nil, errors.Wrap(imgErr, "failed to create image encoder")
	}

	// This is called after initLogging() to avoid a race condition.
	mlog.Info("Server is initializing...", mlog.String("go_version", runtime.Version()))

	// It is important to initialize the hub only after the global logger is set
	// to avoid race conditions while logging from inside the hub.
	app := New(ServerConnector(s))
	app.HubStart()

	if *s.Config().LogSettings.EnableDiagnostics && *s.Config().LogSettings.EnableSentry {
		if strings.Contains(SentryDSN, "placeholder") {
			mlog.Warn("Sentry reporting is enabled, but SENTRY_DSN is not set. Disabling reporting.")
		} else {
			if err := sentry.Init(sentry.ClientOptions{
				Dsn:              SentryDSN,
				Release:          model.BuildHash,
				AttachStacktrace: true,
				BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
					// sanitize data sent to sentry to reduce exposure of PII
					if event.Request != nil {
						event.Request.Cookies = ""
						event.Request.QueryString = ""
						event.Request.Headers = nil
						event.Request.Data = ""
					}
					return event
				},
			}); err != nil {
				mlog.Warn("Sentry could not be initiated, probably bad DSN?", mlog.Err(err))
			}
		}
	}

	if *s.Config().ServiceSettings.EnableOpenTracing {
		tracer, err := tracing.New()
		if err != nil {
			return nil, err
		}
		s.tracer = tracer
	}

	s.HTTPService = httpservice.MakeHTTPService(s)
	s.pushNotificationClient = s.HTTPService.MakeClient(true)

	s.ImageProxy = imageproxy.MakeImageProxy(s, s.HTTPService, s.Log)

	if err := utils.TranslationsPreInit(); err != nil {
		return nil, errors.Wrapf(err, "unable to load Mattermost translation files")
	}
	model.AppErrorInit(i18n.T)

	searchEngine := searchengine.NewBroker(s.Config(), s.Jobs)
	bleveEngine := bleveengine.NewBleveEngine(s.Config(), s.Jobs)
	if err := bleveEngine.Start(); err != nil {
		return nil, err
	}
	searchEngine.RegisterBleveEngine(bleveEngine)
	s.SearchEngine = searchEngine

	// at the moment we only have this implementation
	// in the future the cache provider will be built based on the loaded config
	s.CacheProvider = cache.NewProvider()
	if err := s.CacheProvider.Connect(); err != nil {
		return nil, errors.Wrapf(err, "Unable to connect to cache provider")
	}

	var err error
	if s.seenPendingPostIdsCache, err = s.CacheProvider.NewCache(&cache.CacheOptions{
		Size: PendingPostIDsCacheSize,
	}); err != nil {
		return nil, errors.Wrap(err, "Unable to create pending post ids cache")
	}
	if s.statusCache, err = s.CacheProvider.NewCache(&cache.CacheOptions{
		Size:           model.STATUS_CACHE_SIZE,
		Striped:        true,
		StripedBuckets: maxInt(runtime.NumCPU()-1, 1),
	}); err != nil {
		return nil, errors.Wrap(err, "Unable to create status cache")
	}

	s.createPushNotificationsHub()

	if err2 := i18n.InitTranslations(*s.Config().LocalizationSettings.DefaultServerLocale, *s.Config().LocalizationSettings.DefaultClientLocale); err2 != nil {
		return nil, errors.Wrapf(err2, "unable to load Mattermost translation files")
	}

	s.initEnterprise()

	if s.newStore == nil {
		s.newStore = func() (store.Store, error) {
			s.sqlStore = sqlstore.New(s.Config().SqlSettings, s.Metrics)

			lcl, err2 := localcachelayer.NewLocalCacheLayer(
				retrylayer.New(s.sqlStore),
				s.Metrics,
				s.Cluster,
				s.CacheProvider,
			)
			if err2 != nil {
				return nil, errors.Wrap(err2, "cannot create local cache layer")
			}

			searchStore := searchlayer.NewSearchLayer(
				lcl,
				s.SearchEngine,
				s.Config(),
			)

			s.AddConfigListener(func(prevCfg, cfg *model.Config) {
				searchStore.UpdateConfig(cfg)
			})

			s.sqlStore.UpdateLicense(s.License())
			s.AddLicenseListener(func(oldLicense, newLicense *model.License) {
				s.sqlStore.UpdateLicense(newLicense)
			})

			return timerlayer.New(
				searchStore,
				s.Metrics,
			), nil
		}
	}

	templatesDir, ok := templates.GetTemplateDirectory()
	if !ok {
		mlog.Error("Failed find server templates", mlog.String("directory", "templates"))
	} else {
		htmlTemplateWatcher, errorsChan, err2 := templates.NewWithWatcher(templatesDir)
		if err2 != nil {
			return nil, errors.Wrap(err2, "cannot initialize server templates")
		}
		s.Go(func() {
			for err2 := range errorsChan {
				mlog.Warn("Server templates error", mlog.Err(err2))
			}
		})
		s.htmlTemplateWatcher = htmlTemplateWatcher
	}

	s.Store, err = s.newStore()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create store")
	}

	s.userService, err = users.New(users.ServiceConfig{
		UserStore:    s.Store.User(),
		SessionStore: s.Store.Session(),
		OAuthStore:   s.Store.OAuth(),
		ConfigFn:     s.Config,
		Metrics:      s.Metrics,
		Cluster:      s.Cluster,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to create users service")
	}

	s.configListenerId = s.AddConfigListener(func(_, _ *model.Config) {
		s.configOrLicenseListener()

		message := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_CONFIG_CHANGED, "", "", "", nil)

		message.Add("config", s.ClientConfigWithComputed())
		s.Go(func() {
			s.Publish(message)
		})
	})
	s.licenseListenerId = s.AddLicenseListener(func(oldLicense, newLicense *model.License) {
		s.configOrLicenseListener()

		message := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_LICENSE_CHANGED, "", "", "", nil)
		message.Add("license", s.GetSanitizedClientLicense())
		s.Go(func() {
			s.Publish(message)
		})

	})

	// This enterprise init should happen after the store is set
	// but we don't want to move the s.initEnterprise() call because
	// we had side-effects with that in the past and needs further
	// investigation
	if cloudInterface != nil {
		s.Cloud = cloudInterface(s)
	}

	s.telemetryService = telemetry.New(s, s.Store, s.SearchEngine, s.Log)

	emailService, err := NewEmailService(s)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to initialize email service")
	}
	s.EmailService = emailService

	if model.BuildEnterpriseReady == "true" {
		s.LoadLicense()
	}

	s.setupFeatureFlags()

	s.initJobs()

	s.clusterLeaderListenerId = s.AddClusterLeaderChangedListener(func() {
		mlog.Info("Cluster leader changed. Determining if job schedulers should be running:", mlog.Bool("isLeader", s.IsLeader()))
		if s.Jobs != nil {
			s.Jobs.HandleClusterLeaderChange(s.IsLeader())
		}
		s.setupFeatureFlags()
	})

	if s.joinCluster && s.Cluster != nil {
		s.registerClusterHandlers()
		s.Cluster.StartInterNodeCommunication()
	}

	if err = s.ensureAsymmetricSigningKey(); err != nil {
		return nil, errors.Wrapf(err, "unable to ensure asymmetric signing key")
	}

	if err = s.ensurePostActionCookieSecret(); err != nil {
		return nil, errors.Wrapf(err, "unable to ensure PostAction cookie secret")
	}

	if err = s.ensureInstallationDate(); err != nil {
		return nil, errors.Wrapf(err, "unable to ensure installation date")
	}

	if err = s.ensureFirstServerRunTimestamp(); err != nil {
		return nil, errors.Wrapf(err, "unable to ensure first run timestamp")
	}

	s.regenerateClientConfig()

	subpath, err := utils.GetSubpathFromConfig(s.Config())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse SiteURL subpath")
	}
	s.Router = s.RootRouter.PathPrefix(subpath).Subrouter()

	pluginsRoute := s.Router.PathPrefix("/plugins/{plugin_id:[A-Za-z0-9\\_\\-\\.]+}").Subrouter()
	pluginsRoute.HandleFunc("", s.ServePluginRequest)
	pluginsRoute.HandleFunc("/public/{public_file:.*}", s.ServePluginPublicRequest)
	pluginsRoute.HandleFunc("/{anything:.*}", s.ServePluginRequest)

	// If configured with a subpath, redirect 404s at the root back into the subpath.
	if subpath != "/" {
		s.RootRouter.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = path.Join(subpath, r.URL.Path)
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
		})
	}

	s.WebSocketRouter = &WebSocketRouter{
		handlers: make(map[string]webSocketHandler),
		app:      app,
	}

	mailConfig := s.MailServiceConfig()

	if nErr := mail.TestConnection(mailConfig); nErr != nil {
		mlog.Error("Mail server connection test is failed", mlog.Err(nErr))
	}

	if _, err = url.ParseRequestURI(*s.Config().ServiceSettings.SiteURL); err != nil {
		mlog.Error("SiteURL must be set. Some features will operate incorrectly if the SiteURL is not set. See documentation for details: http://about.mattermost.com/default-site-url")
	}

	backend, appErr := s.FileBackend()
	if appErr != nil {
		mlog.Error("Problem with file storage settings", mlog.Err(appErr))
	} else {
		nErr := backend.TestConnection()
		if nErr != nil {
			if _, ok := nErr.(*filestore.S3FileBackendNoBucketError); ok {
				nErr = backend.(*filestore.S3FileBackend).MakeBucket()
			}
			if nErr != nil {
				mlog.Error("Problem with file storage settings", mlog.Err(nErr))
			}
		}
	}

	s.timezones = timezones.New()
	// Start email batching because it's not like the other jobs
	s.AddConfigListener(func(_, _ *model.Config) {
		s.EmailService.InitEmailBatching()
	})

	// Start plugin health check job
	pluginsEnvironment := s.PluginsEnvironment
	if pluginsEnvironment != nil {
		pluginsEnvironment.InitPluginHealthCheckJob(*s.Config().PluginSettings.Enable && *s.Config().PluginSettings.EnableHealthCheck)
	}
	s.AddConfigListener(func(_, c *model.Config) {
		s.PluginsLock.RLock()
		pluginsEnvironment := s.PluginsEnvironment
		s.PluginsLock.RUnlock()
		if pluginsEnvironment != nil {
			pluginsEnvironment.InitPluginHealthCheckJob(*s.Config().PluginSettings.Enable && *c.PluginSettings.EnableHealthCheck)
		}
	})

	logCurrentVersion := fmt.Sprintf("Current version is %v (%v/%v/%v/%v)", model.CurrentVersion, model.BuildNumber, model.BuildDate, model.BuildHash, model.BuildHashEnterprise)
	mlog.Info(
		logCurrentVersion,
		mlog.String("current_version", model.CurrentVersion),
		mlog.String("build_number", model.BuildNumber),
		mlog.String("build_date", model.BuildDate),
		mlog.String("build_hash", model.BuildHash),
		mlog.String("build_hash_enterprise", model.BuildHashEnterprise),
	)
	if model.BuildEnterpriseReady == "true" {
		mlog.Info("Enterprise Build", mlog.Bool("enterprise_build", true))
	} else {
		mlog.Info("Team Edition Build", mlog.Bool("enterprise_build", false))
	}

	pwd, _ := os.Getwd()
	mlog.Info("Printing current working", mlog.String("directory", pwd))
	mlog.Info("Loaded config", mlog.String("source", s.configStore.String()))

	s.checkPushNotificationServerUrl()

	license := s.License()
	if license == nil {
		s.UpdateConfig(func(cfg *model.Config) {
			cfg.TeamSettings.MaxNotificationsPerChannel = &MaxNotificationsPerChannelDefault
		})
	}

	s.ReloadConfig()

	allowAdvancedLogging := license != nil && *license.Features.AdvancedLogging

	if s.Audit == nil {
		s.Audit = &audit.Audit{}
		s.Audit.Init(audit.DefMaxQueueSize)
		if err = s.configureAudit(s.Audit, allowAdvancedLogging); err != nil {
			mlog.Error("Error configuring audit", mlog.Err(err))
		}
	}

	s.removeUnlicensedLogTargets(license)
	s.enableLoggingMetrics()

	s.loggerLicenseListenerId = s.AddLicenseListener(func(oldLicense, newLicense *model.License) {
		s.removeUnlicensedLogTargets(newLicense)
		s.enableLoggingMetrics()
	})

	// Enable developer settings if this is a "dev" build
	if model.BuildNumber == "dev" {
		s.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableDeveloper = true })
	}

	if err = s.Store.Status().ResetAll(); err != nil {
		mlog.Error("Error to reset the server status.", mlog.Err(err))
	}

	if s.startMetrics {
		s.SetupMetricsServer()
	}

	s.AddLicenseListener(func(oldLicense, newLicense *model.License) {
		if (oldLicense == nil && newLicense == nil) || !s.startMetrics {
			return
		}

		if oldLicense != nil && newLicense != nil && *oldLicense.Features.Metrics == *newLicense.Features.Metrics {
			return
		}

		s.SetupMetricsServer()
	})

	s.SearchEngine.UpdateConfig(s.Config())
	searchConfigListenerId, searchLicenseListenerId := s.StartSearchEngine()
	s.searchConfigListenerId = searchConfigListenerId
	s.searchLicenseListenerId = searchLicenseListenerId

	// if enabled - perform initial product notices fetch
	if *s.Config().AnnouncementSettings.AdminNoticesEnabled || *s.Config().AnnouncementSettings.UserNoticesEnabled {
		go func() {
			if err := app.UpdateProductNotices(); err != nil {
				mlog.Warn("Failied to perform initial product notices fetch", mlog.Err(err))
			}
		}()
	}

	if s.skipPostInit {
		return s, nil
	}

	c := request.EmptyContext()
	s.AddConfigListener(func(oldConfig *model.Config, newConfig *model.Config) {
		if *oldConfig.GuestAccountsSettings.Enable && !*newConfig.GuestAccountsSettings.Enable {
			if appErr := app.DeactivateGuests(c); appErr != nil {
				mlog.Error("Unable to deactivate guest accounts", mlog.Err(appErr))
			}
		}
	})

	// Disable active guest accounts on first run if guest accounts are disabled
	if !*s.Config().GuestAccountsSettings.Enable {
		if appErr := app.DeactivateGuests(c); appErr != nil {
			mlog.Error("Unable to deactivate guest accounts", mlog.Err(appErr))
		}
	}

	if s.runEssentialJobs {
		s.Go(func() {
			s.runLicenseExpirationCheckJob()
			runCheckAdminSupportStatusJob(app, c)
			runCheckWarnMetricStatusJob(app, c)
			runDNDStatusExpireJob(app)
		})
		s.runJobs()
	}

	s.doAppMigrations()

	s.initPostMetadata()

	s.initPlugins(c, *s.Config().PluginSettings.Directory, *s.Config().PluginSettings.ClientDirectory)
	s.AddConfigListener(func(prevCfg, cfg *model.Config) {
		if *cfg.PluginSettings.Enable {
			s.initPlugins(c, *cfg.PluginSettings.Directory, *s.Config().PluginSettings.ClientDirectory)
		} else {
			s.ShutDownPlugins()
		}
	})
	s.AddConfigListener(func(oldCfg, newCfg *model.Config) {
		if !oldCfg.FeatureFlags.TimedDND && newCfg.FeatureFlags.TimedDND {
			runDNDStatusExpireJob(app)
		}
		if oldCfg.FeatureFlags.TimedDND && !newCfg.FeatureFlags.TimedDND {
			stopDNDStatusExpireJob(app)
		}
	})

	return s, nil
}

func (s *Server) SetupMetricsServer() {
	if !*s.Config().MetricsSettings.Enable {
		return
	}

	s.StopMetricsServer()

	if err := s.InitMetricsRouter(); err != nil {
		mlog.Error("Error initiating metrics router.", mlog.Err(err))
	}

	if s.Metrics != nil {
		s.Metrics.Register()
	}

	s.startMetricsServer()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Server) runJobs() {
	s.Go(func() {
		runSecurityJob(s)
	})
	s.Go(func() {
		firstRun, err := s.getFirstServerRunTimestamp()
		if err != nil {
			mlog.Warn("Fetching time of first server run failed. Setting to 'now'.")
			s.ensureFirstServerRunTimestamp()
			firstRun = utils.MillisFromTime(time.Now())
		}
		s.telemetryService.RunTelemetryJob(firstRun)
	})
	s.Go(func() {
		runSessionCleanupJob(s)
	})
	s.Go(func() {
		runTokenCleanupJob(s)
	})
	s.Go(func() {
		runCommandWebhookCleanupJob(s)
	})

	if complianceI := s.Compliance; complianceI != nil {
		complianceI.StartComplianceDailyJob()
	}

	if *s.Config().JobSettings.RunJobs && s.Jobs != nil {
		if err := s.Jobs.StartWorkers(); err != nil {
			mlog.Error("Failed to start job server workers", mlog.Err(err))
		}
	}
	if *s.Config().JobSettings.RunScheduler && s.Jobs != nil {
		if err := s.Jobs.StartSchedulers(); err != nil {
			mlog.Error("Failed to start job server schedulers", mlog.Err(err))
		}
	}

	if *s.Config().ServiceSettings.EnableAWSMetering {
		runReportToAWSMeterJob(s)
	}
}

// Global app options that should be applied to apps created by this server
func (s *Server) AppOptions() []AppOption {
	return []AppOption{
		ServerConnector(s),
	}
}

// Return Database type (postgres or mysql) and current version of Mattermost
func (s *Server) DatabaseTypeAndMattermostVersion() (string, string) {
	mattermostVersion, _ := s.Store.System().GetByName("Version")
	return *s.Config().SqlSettings.DriverName, mattermostVersion.Value
}

// initLogging initializes and configures the logger. This may be called more than once.
func (s *Server) initLogging() error {
	if s.Log == nil {
		s.Log = mlog.NewLogger(utils.MloggerConfigFromLoggerConfig(&s.Config().LogSettings, utils.GetLogFileLocation))
	}

	// Use this app logger as the global logger (eventually remove all instances of global logging).
	// This is deferred because a copy is made of the logger and it must be fully configured before
	// the copy is made.
	defer mlog.InitGlobalLogger(s.Log)

	// Redirect default Go logger to this logger.
	defer mlog.RedirectStdLog(s.Log)

	if s.NotificationsLog == nil {
		notificationLogSettings := utils.GetLogSettingsFromNotificationsLogSettings(&s.Config().NotificationLogSettings)
		s.NotificationsLog = mlog.NewLogger(utils.MloggerConfigFromLoggerConfig(notificationLogSettings, utils.GetNotificationsLogFileLocation)).
			WithCallerSkip(1).With(mlog.String("logSource", "notifications"))
	}

	if s.logListenerId != "" {
		s.RemoveConfigListener(s.logListenerId)
	}
	s.logListenerId = s.AddConfigListener(func(_, after *model.Config) {
		s.Log.ChangeLevels(utils.MloggerConfigFromLoggerConfig(&after.LogSettings, utils.GetLogFileLocation))

		notificationLogSettings := utils.GetLogSettingsFromNotificationsLogSettings(&after.NotificationLogSettings)
		s.NotificationsLog.ChangeLevels(utils.MloggerConfigFromLoggerConfig(notificationLogSettings, utils.GetNotificationsLogFileLocation))
	})

	// Configure advanced logging.
	// Advanced logging is E20 only, however logging must be initialized before the license
	// file is loaded.  If no valid E20 license exists then advanced logging will be
	// shutdown once license is loaded/checked.
	if *s.Config().LogSettings.AdvancedLoggingConfig != "" {
		dsn := *s.Config().LogSettings.AdvancedLoggingConfig

		cfg, err := config.NewLogConfigSrc(dsn, s.configStore)
		if err != nil {
			return fmt.Errorf("invalid advanced logging config, %w", err)
		}

		if err := s.Log.ConfigAdvancedLogging(cfg.Get()); err != nil {
			return fmt.Errorf("error configuring advanced logging, %w", err)
		}

		mlog.Info("Loaded advanced logging config", mlog.String("source", dsn))

		listenerId := cfg.AddListener(func(_, newCfg mlog.LogTargetCfg) {
			if err := s.Log.ConfigAdvancedLogging(newCfg); err != nil {
				mlog.Error("Error re-configuring advanced logging", mlog.Err(err))
			} else {
				mlog.Info("Re-configured advanced logging")
			}
		})

		// In case initLogging is called more than once.
		if s.advancedLogListenerCleanup != nil {
			s.advancedLogListenerCleanup()
		}

		s.advancedLogListenerCleanup = func() {
			cfg.RemoveListener(listenerId)
		}
	}
	return nil
}

func (s *Server) removeUnlicensedLogTargets(license *model.License) {
	if license != nil && *license.Features.AdvancedLogging {
		// advanced logging enabled via license; no need to remove any targets
		return
	}

	timeoutCtx, cancelCtx := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelCtx()

	mlog.RemoveTargets(timeoutCtx, func(ti mlog.TargetInfo) bool {
		return ti.Type != "*target.Writer" && ti.Type != "*target.File"
	})
}

func (s *Server) startInterClusterServices(license *model.License, app *App) error {
	if license == nil {
		mlog.Debug("No license provided; Remote Cluster services disabled")
		return nil
	}

	// Remote Cluster service

	// License check
	if !*license.Features.RemoteClusterService {
		mlog.Debug("License does not have Remote Cluster services enabled")
		return nil
	}

	// Config check
	if !*s.Config().ExperimentalSettings.EnableRemoteClusterService {
		mlog.Debug("Remote Cluster Service disabled via config")
		return nil
	}

	var err error

	rcs, err := remotecluster.NewRemoteClusterService(s)
	if err != nil {
		return err
	}

	if err = rcs.Start(); err != nil {
		return err
	}

	s.serviceMux.Lock()
	s.remoteClusterService = rcs
	s.serviceMux.Unlock()

	// Shared Channels service

	// License check
	if !*license.Features.SharedChannels {
		mlog.Debug("License does not have shared channels enabled")
		return nil
	}

	// Config check
	if !*s.Config().ExperimentalSettings.EnableSharedChannels {
		mlog.Debug("Shared Channels Service disabled via config")
		return nil
	}

	scs, err := sharedchannel.NewSharedChannelService(s, app)
	if err != nil {
		return err
	}

	if err = scs.Start(); err != nil {
		return err
	}

	s.serviceMux.Lock()
	s.sharedChannelService = scs
	s.serviceMux.Unlock()

	return nil
}

func (s *Server) enableLoggingMetrics() {
	if s.Metrics == nil {
		return
	}

	if err := mlog.EnableMetrics(s.Metrics.GetLoggerMetricsCollector()); err != nil {
		mlog.Error("Failed to enable advanced logging metrics", mlog.Err(err))
	} else {
		mlog.Debug("Advanced logging metrics enabled")
	}
}

const TimeToWaitForConnectionsToCloseOnServerShutdown = time.Second

func (s *Server) StopHTTPServer() {
	if s.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), TimeToWaitForConnectionsToCloseOnServerShutdown)
		defer cancel()
		didShutdown := false
		for s.didFinishListen != nil && !didShutdown {
			if err := s.Server.Shutdown(ctx); err != nil {
				mlog.Warn("Unable to shutdown server", mlog.Err(err))
			}
			timer := time.NewTimer(time.Millisecond * 50)
			select {
			case <-s.didFinishListen:
				didShutdown = true
			case <-timer.C:
			}
			timer.Stop()
		}
		s.Server.Close()
		s.Server = nil
	}
}

func (s *Server) Shutdown() {
	mlog.Info("Stopping Server...")

	defer sentry.Flush(2 * time.Second)

	s.HubStop()
	s.ShutDownPlugins()
	s.RemoveLicenseListener(s.licenseListenerId)
	s.RemoveLicenseListener(s.loggerLicenseListenerId)
	s.RemoveClusterLeaderChangedListener(s.clusterLeaderListenerId)

	if s.tracer != nil {
		if err := s.tracer.Close(); err != nil {
			mlog.Warn("Unable to cleanly shutdown opentracing client", mlog.Err(err))
		}
	}

	err := s.telemetryService.Shutdown()
	if err != nil {
		mlog.Warn("Unable to cleanly shutdown telemetry client", mlog.Err(err))
	}

	s.serviceMux.RLock()
	if s.sharedChannelService != nil {
		if err = s.sharedChannelService.Shutdown(); err != nil {
			mlog.Error("Error shutting down shared channel services", mlog.Err(err))
		}
	}
	if s.remoteClusterService != nil {
		if err = s.remoteClusterService.Shutdown(); err != nil {
			mlog.Error("Error shutting down intercluster services", mlog.Err(err))
		}
	}
	s.serviceMux.RUnlock()

	s.StopHTTPServer()
	s.stopLocalModeServer()
	// Push notification hub needs to be shutdown after HTTP server
	// to prevent stray requests from generating a push notification after it's shut down.
	s.StopPushNotificationsHubWorkers()
	s.htmlTemplateWatcher.Close()

	s.WaitForGoroutines()

	if s.advancedLogListenerCleanup != nil {
		s.advancedLogListenerCleanup()
		s.advancedLogListenerCleanup = nil
	}

	s.RemoveConfigListener(s.configListenerId)
	s.RemoveConfigListener(s.logListenerId)
	s.stopSearchEngine()

	s.Audit.Shutdown()

	s.stopFeatureFlagUpdateJob()

	s.configStore.Close()

	if s.Cluster != nil {
		s.Cluster.StopInterNodeCommunication()
	}

	s.StopMetricsServer()

	// This must be done after the cluster is stopped.
	if s.Jobs != nil {
		// For simplicity we don't check if workers and schedulers are active
		// before stopping them as both calls essentially become no-ops
		// if nothing is running.
		if err = s.Jobs.StopWorkers(); err != nil && !errors.Is(err, jobs.ErrWorkersNotRunning) {
			mlog.Warn("Failed to stop job server workers", mlog.Err(err))
		}
		if err = s.Jobs.StopSchedulers(); err != nil && !errors.Is(err, jobs.ErrSchedulersNotRunning) {
			mlog.Warn("Failed to stop job server schedulers", mlog.Err(err))
		}
	}

	if s.Store != nil {
		s.Store.Close()
	}

	if s.CacheProvider != nil {
		if err = s.CacheProvider.Close(); err != nil {
			mlog.Warn("Unable to cleanly shutdown cache", mlog.Err(err))
		}
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*15)
	defer timeoutCancel()
	if err := mlog.Flush(timeoutCtx); err != nil {
		mlog.Warn("Error flushing logs", mlog.Err(err))
	}

	s.dndTaskMut.Lock()
	if s.dndTask != nil {
		s.dndTask.Cancel()
	}
	s.dndTaskMut.Unlock()

	mlog.Info("Server stopped")

	// this should just write the "server stopped" record, the rest are already flushed.
	timeoutCtx2, timeoutCancel2 := context.WithTimeout(context.Background(), time.Second*5)
	defer timeoutCancel2()
	_ = mlog.ShutdownAdvancedLogging(timeoutCtx2)
}

func (s *Server) Restart() error {
	percentage, err := s.UpgradeToE0Status()
	if err != nil || percentage != 100 {
		return errors.Wrap(err, "unable to restart because the system has not been upgraded")
	}
	s.Shutdown()

	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}

	if _, err = os.Stat(argv0); err != nil {
		return err
	}

	mlog.Info("Restarting server")
	return syscall.Exec(argv0, os.Args, os.Environ())
}

func (s *Server) isUpgradedFromTE() bool {
	val, err := s.Store.System().GetByName(model.SYSTEM_UPGRADED_FROM_TE_ID)
	if err != nil {
		return false
	}
	return val.Value == "true"
}

func (s *Server) CanIUpgradeToE0() error {
	return upgrader.CanIUpgradeToE0()
}

func (s *Server) UpgradeToE0() error {
	if err := upgrader.UpgradeToE0(); err != nil {
		return err
	}
	upgradedFromTE := &model.System{Name: model.SYSTEM_UPGRADED_FROM_TE_ID, Value: "true"}
	s.Store.System().Save(upgradedFromTE)
	return nil
}

func (s *Server) UpgradeToE0Status() (int64, error) {
	return upgrader.UpgradeToE0Status()
}

// Go creates a goroutine, but maintains a record of it to ensure that execution completes before
// the server is shutdown.
func (s *Server) Go(f func()) {
	atomic.AddInt32(&s.goroutineCount, 1)

	go func() {
		f()

		atomic.AddInt32(&s.goroutineCount, -1)
		select {
		case s.goroutineExitSignal <- struct{}{}:
		default:
		}
	}()
}

// WaitForGoroutines blocks until all goroutines created by App.Go exit.
func (s *Server) WaitForGoroutines() {
	for atomic.LoadInt32(&s.goroutineCount) != 0 {
		<-s.goroutineExitSignal
	}
}

var corsAllowedMethods = []string{
	"POST",
	"GET",
	"OPTIONS",
	"PUT",
	"PATCH",
	"DELETE",
}

// golang.org/x/crypto/acme/autocert/autocert.go
func handleHTTPRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}
	target := "https://" + stripPort(r.Host) + r.URL.RequestURI()
	http.Redirect(w, r, target, http.StatusFound)
}

// golang.org/x/crypto/acme/autocert/autocert.go
func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return net.JoinHostPort(host, "443")
}

func (s *Server) Start() error {
	mlog.Info("Starting Server...")

	var handler http.Handler = s.RootRouter

	if *s.Config().LogSettings.EnableDiagnostics && *s.Config().LogSettings.EnableSentry && !strings.Contains(SentryDSN, "placeholder") {
		sentryHandler := sentryhttp.New(sentryhttp.Options{
			Repanic: true,
		})
		handler = sentryHandler.Handle(handler)
	}

	if allowedOrigins := *s.Config().ServiceSettings.AllowCorsFrom; allowedOrigins != "" {
		exposedCorsHeaders := *s.Config().ServiceSettings.CorsExposedHeaders
		allowCredentials := *s.Config().ServiceSettings.CorsAllowCredentials
		debug := *s.Config().ServiceSettings.CorsDebug
		corsWrapper := cors.New(cors.Options{
			AllowedOrigins:   strings.Fields(allowedOrigins),
			AllowedMethods:   corsAllowedMethods,
			AllowedHeaders:   []string{"*"},
			ExposedHeaders:   strings.Fields(exposedCorsHeaders),
			MaxAge:           86400,
			AllowCredentials: allowCredentials,
			Debug:            debug,
		})

		// If we have debugging of CORS turned on then forward messages to logs
		if debug {
			corsWrapper.Log = s.Log.StdLog(mlog.String("source", "cors"))
		}

		handler = corsWrapper.Handler(handler)
	}

	if *s.Config().RateLimitSettings.Enable {
		mlog.Info("RateLimiter is enabled")

		rateLimiter, err := NewRateLimiter(&s.Config().RateLimitSettings, s.Config().ServiceSettings.TrustedProxyIPHeader)
		if err != nil {
			return err
		}

		s.RateLimiter = rateLimiter
		handler = rateLimiter.RateLimitHandler(handler)
	}
	s.Busy = NewBusy(s.Cluster)

	// Creating a logger for logging errors from http.Server at error level
	errStdLog, err := s.Log.StdLogAt(mlog.LevelError, mlog.String("source", "httpserver"))
	if err != nil {
		return err
	}

	s.Server = &http.Server{
		Handler:      handler,
		ReadTimeout:  time.Duration(*s.Config().ServiceSettings.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(*s.Config().ServiceSettings.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(*s.Config().ServiceSettings.IdleTimeout) * time.Second,
		ErrorLog:     errStdLog,
	}

	addr := *s.Config().ServiceSettings.ListenAddress
	if addr == "" {
		if *s.Config().ServiceSettings.ConnectionSecurity == model.CONN_SECURITY_TLS {
			addr = ":https"
		} else {
			addr = ":http"
		}
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrapf(err, i18n.T("api.server.start_server.starting.critical"), err)
	}
	s.ListenAddr = listener.Addr().(*net.TCPAddr)

	logListeningPort := fmt.Sprintf("Server is listening on %v", listener.Addr().String())
	mlog.Info(logListeningPort, mlog.String("address", listener.Addr().String()))

	m := &autocert.Manager{
		Cache:  autocert.DirCache(*s.Config().ServiceSettings.LetsEncryptCertificateCacheFile),
		Prompt: autocert.AcceptTOS,
	}

	if *s.Config().ServiceSettings.Forward80To443 {
		if host, port, err := net.SplitHostPort(addr); err != nil {
			mlog.Error("Unable to setup forwarding", mlog.Err(err))
		} else if port != "443" {
			return fmt.Errorf(i18n.T("api.server.start_server.forward80to443.enabled_but_listening_on_wrong_port"), port)
		} else {
			httpListenAddress := net.JoinHostPort(host, "http")

			if *s.Config().ServiceSettings.UseLetsEncrypt {
				server := &http.Server{
					Addr:     httpListenAddress,
					Handler:  m.HTTPHandler(nil),
					ErrorLog: s.Log.StdLog(mlog.String("source", "le_forwarder_server")),
				}
				go server.ListenAndServe()
			} else {
				go func() {
					redirectListener, err := net.Listen("tcp", httpListenAddress)
					if err != nil {
						mlog.Error("Unable to setup forwarding", mlog.Err(err))
						return
					}
					defer redirectListener.Close()

					server := &http.Server{
						Handler:  http.HandlerFunc(handleHTTPRedirect),
						ErrorLog: s.Log.StdLog(mlog.String("source", "forwarder_server")),
					}
					server.Serve(redirectListener)
				}()
			}
		}
	} else if *s.Config().ServiceSettings.UseLetsEncrypt {
		return errors.New(i18n.T("api.server.start_server.forward80to443.disabled_while_using_lets_encrypt"))
	}

	s.didFinishListen = make(chan struct{})
	go func() {
		var err error
		if *s.Config().ServiceSettings.ConnectionSecurity == model.CONN_SECURITY_TLS {

			tlsConfig := &tls.Config{
				PreferServerCipherSuites: true,
				CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			}

			switch *s.Config().ServiceSettings.TLSMinVer {
			case "1.0":
				tlsConfig.MinVersion = tls.VersionTLS10
			case "1.1":
				tlsConfig.MinVersion = tls.VersionTLS11
			default:
				tlsConfig.MinVersion = tls.VersionTLS12
			}

			defaultCiphers := []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			}

			if len(s.Config().ServiceSettings.TLSOverwriteCiphers) == 0 {
				tlsConfig.CipherSuites = defaultCiphers
			} else {
				var cipherSuites []uint16
				for _, cipher := range s.Config().ServiceSettings.TLSOverwriteCiphers {
					value, ok := model.ServerTLSSupportedCiphers[cipher]

					if !ok {
						mlog.Warn("Unsupported cipher passed", mlog.String("cipher", cipher))
						continue
					}

					cipherSuites = append(cipherSuites, value)
				}

				if len(cipherSuites) == 0 {
					mlog.Warn("No supported ciphers passed, fallback to default cipher suite")
					cipherSuites = defaultCiphers
				}

				tlsConfig.CipherSuites = cipherSuites
			}

			certFile := ""
			keyFile := ""

			if *s.Config().ServiceSettings.UseLetsEncrypt {
				tlsConfig.GetCertificate = m.GetCertificate
				tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")
			} else {
				certFile = *s.Config().ServiceSettings.TLSCertFile
				keyFile = *s.Config().ServiceSettings.TLSKeyFile
			}

			s.Server.TLSConfig = tlsConfig
			err = s.Server.ServeTLS(listener, certFile, keyFile)
		} else {
			err = s.Server.Serve(listener)
		}

		if err != nil && err != http.ErrServerClosed {
			mlog.Critical("Error starting server", mlog.Err(err))
			time.Sleep(time.Second)
		}

		close(s.didFinishListen)
	}()

	if *s.Config().ServiceSettings.EnableLocalMode {
		if err := s.startLocalModeServer(); err != nil {
			mlog.Critical(err.Error())
		}
	}

	if err := s.startInterClusterServices(s.License(), s.WebSocketRouter.app); err != nil {
		mlog.Error("Error starting inter-cluster services", mlog.Err(err))
	}

	return nil
}

func (s *Server) startLocalModeServer() error {
	s.localModeServer = &http.Server{
		Handler: s.LocalRouter,
	}

	socket := *s.configStore.Get().ServiceSettings.LocalModeSocketLocation
	if err := os.RemoveAll(socket); err != nil {
		return errors.Wrapf(err, i18n.T("api.server.start_server.starting.critical"), err)
	}

	unixListener, err := net.Listen("unix", socket)
	if err != nil {
		return errors.Wrapf(err, i18n.T("api.server.start_server.starting.critical"), err)
	}
	if err = os.Chmod(socket, 0600); err != nil {
		return errors.Wrapf(err, i18n.T("api.server.start_server.starting.critical"), err)
	}

	go func() {
		err = s.localModeServer.Serve(unixListener)
		if err != nil && err != http.ErrServerClosed {
			mlog.Critical("Error starting unix socket server", mlog.Err(err))
		}
	}()
	return nil
}

func (s *Server) stopLocalModeServer() {
	if s.localModeServer != nil {
		s.localModeServer.Close()
	}
}

func (a *App) OriginChecker() func(*http.Request) bool {
	if allowed := *a.Config().ServiceSettings.AllowCorsFrom; allowed != "" {
		if allowed != "*" {
			siteURL, err := url.Parse(*a.Config().ServiceSettings.SiteURL)
			if err == nil {
				siteURL.Path = ""
				allowed += " " + siteURL.String()
			}
		}

		return utils.OriginChecker(allowed)
	}
	return nil
}

func (s *Server) checkPushNotificationServerUrl() {
	notificationServer := *s.Config().EmailSettings.PushNotificationServer
	if strings.HasPrefix(notificationServer, "http://") {
		mlog.Warn("Your push notification server is configured with HTTP. For improved security, update to HTTPS in your configuration.")
	}
}

func runSecurityJob(s *Server) {
	doSecurity(s)
	model.CreateRecurringTask("Security", func() {
		doSecurity(s)
	}, time.Hour*4)
}

func runTokenCleanupJob(s *Server) {
	doTokenCleanup(s)
	model.CreateRecurringTask("Token Cleanup", func() {
		doTokenCleanup(s)
	}, time.Hour*1)
}

func runCommandWebhookCleanupJob(s *Server) {
	doCommandWebhookCleanup(s)
	model.CreateRecurringTask("Command Hook Cleanup", func() {
		doCommandWebhookCleanup(s)
	}, time.Hour*1)
}

func runSessionCleanupJob(s *Server) {
	doSessionCleanup(s)
	model.CreateRecurringTask("Session Cleanup", func() {
		doSessionCleanup(s)
	}, time.Hour*24)
}

func (s *Server) runLicenseExpirationCheckJob() {
	s.doLicenseExpirationCheck()
	model.CreateRecurringTask("License Expiration Check", func() {
		s.doLicenseExpirationCheck()
	}, time.Hour*24)
}

func runReportToAWSMeterJob(s *Server) {
	model.CreateRecurringTask("Collect and send usage report to AWS Metering Service", func() {
		doReportUsageToAWSMeteringService(s)
	}, time.Hour*model.AWS_METERING_REPORT_INTERVAL)
}

func doReportUsageToAWSMeteringService(s *Server) {
	awsMeter := awsmeter.New(s.Store, s.Config())
	if awsMeter == nil {
		mlog.Error("Cannot obtain instance of AWS Metering Service.")
		return
	}

	dimensions := []string{model.AWS_METERING_DIMENSION_USAGE_HRS}
	reports := awsMeter.GetUserCategoryUsage(dimensions, time.Now().UTC(), time.Now().Add(-model.AWS_METERING_REPORT_INTERVAL*time.Hour).UTC())
	awsMeter.ReportUserCategoryUsage(reports)
}

//nolint:golint,unused,deadcode
func runCheckWarnMetricStatusJob(a *App, c *request.Context) {
	doCheckWarnMetricStatus(a, c)
	model.CreateRecurringTask("Check Warn Metric Status Job", func() {
		doCheckWarnMetricStatus(a, c)
	}, time.Hour*model.WARN_METRIC_JOB_INTERVAL)
}

func runCheckAdminSupportStatusJob(a *App, c *request.Context) {
	doCheckAdminSupportStatus(a, c)
	model.CreateRecurringTask("Check Admin Support Status Job", func() {
		doCheckAdminSupportStatus(a, c)
	}, time.Hour*model.WARN_METRIC_JOB_INTERVAL)
}

func doSecurity(s *Server) {
	s.DoSecurityUpdateCheck()
}

func doTokenCleanup(s *Server) {
	s.Store.Token().Cleanup()
}

func doCommandWebhookCleanup(s *Server) {
	s.Store.CommandWebhook().Cleanup()
}

const (
	SessionsCleanupBatchSize = 1000
)

func doSessionCleanup(s *Server) {
	s.Store.Session().Cleanup(model.GetMillis(), SessionsCleanupBatchSize)
}

//nolint:golint,unused,deadcode
func doCheckWarnMetricStatus(a *App, c *request.Context) {
	license := a.Srv().License()
	if license != nil {
		mlog.Debug("License is present, skip")
		return
	}

	// Get the system fields values from store
	systemDataList, nErr := a.Srv().Store.System().Get()
	if nErr != nil {
		mlog.Error("No system properties obtained", mlog.Err(nErr))
		return
	}

	warnMetricStatusFromStore := make(map[string]string)

	for key, value := range systemDataList {
		if strings.HasPrefix(key, model.WARN_METRIC_STATUS_STORE_PREFIX) {
			if _, ok := model.WarnMetricsTable[key]; ok {
				warnMetricStatusFromStore[key] = value
				if value == model.WARN_METRIC_STATUS_ACK {
					// If any warn metric has already been acked, we return
					mlog.Debug("Warn metrics have been acked, skip")
					return
				}
			}
		}
	}

	lastWarnMetricRunTimestamp, err := a.Srv().getLastWarnMetricTimestamp()
	if err != nil {
		mlog.Debug("Cannot obtain last advisory run timestamp", mlog.Err(err))
	} else {
		currentTime := utils.MillisFromTime(time.Now())
		// If the admin advisory has already been shown in the last 7 days
		if (currentTime-lastWarnMetricRunTimestamp)/(model.WARN_METRIC_JOB_WAIT_TIME) < 1 {
			mlog.Debug("No advisories should be shown during the wait interval time")
			return
		}
	}

	numberOfActiveUsers, err0 := a.Srv().Store.User().Count(model.UserCountOptions{})
	if err0 != nil {
		mlog.Debug("Error attempting to get active registered users.", mlog.Err(err0))
	}

	teamCount, err1 := a.Srv().Store.Team().AnalyticsTeamCount(nil)
	if err1 != nil {
		mlog.Debug("Error attempting to get number of teams.", mlog.Err(err1))
	}

	openChannelCount, err2 := a.Srv().Store.Channel().AnalyticsTypeCount("", model.CHANNEL_OPEN)
	if err2 != nil {
		mlog.Debug("Error attempting to get number of public channels.", mlog.Err(err2))
	}

	// If an account is created with a different email domain
	// Search for an entry that has an email account different from the current domain
	// Get domain account from site url
	localDomainAccount := utils.GetHostnameFromSiteURL(*a.Srv().Config().ServiceSettings.SiteURL)
	isDiffEmailAccount, err3 := a.Srv().Store.User().AnalyticsGetExternalUsers(localDomainAccount)
	if err3 != nil {
		mlog.Debug("Error attempting to get number of private channels.", mlog.Err(err3))
	}

	warnMetrics := []model.WarnMetric{}

	if numberOfActiveUsers < model.WARN_METRIC_NUMBER_OF_ACTIVE_USERS_25 {
		return
	} else if teamCount >= model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_TEAMS_5].Limit && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_TEAMS_5] != model.WARN_METRIC_STATUS_RUNONCE {
		warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_TEAMS_5])
	} else if *a.Config().ServiceSettings.EnableMultifactorAuthentication && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_MFA] != model.WARN_METRIC_STATUS_RUNONCE {
		warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_MFA])
	} else if isDiffEmailAccount && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_EMAIL_DOMAIN] != model.WARN_METRIC_STATUS_RUNONCE {
		warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_EMAIL_DOMAIN])
	} else if openChannelCount >= model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_CHANNELS_50].Limit && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_CHANNELS_50] != model.WARN_METRIC_STATUS_RUNONCE {
		warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_CHANNELS_50])
	}

	// If the system did not cross any of the thresholds for the Contextual Advisories
	if len(warnMetrics) == 0 {
		if numberOfActiveUsers >= model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_100].Limit && numberOfActiveUsers < model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_200].Limit && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_100] != model.WARN_METRIC_STATUS_RUNONCE {
			warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_100])
		} else if numberOfActiveUsers >= model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_200].Limit && numberOfActiveUsers < model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_300].Limit && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_200] != model.WARN_METRIC_STATUS_RUNONCE {
			warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_200])
		} else if numberOfActiveUsers >= model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_300].Limit && numberOfActiveUsers < model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_500].Limit && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_300] != model.WARN_METRIC_STATUS_RUNONCE {
			warnMetrics = append(warnMetrics, model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_300])
		} else if numberOfActiveUsers >= model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_500].Limit {
			var tWarnMetric model.WarnMetric

			if warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_500] != model.WARN_METRIC_STATUS_RUNONCE {
				tWarnMetric = model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_ACTIVE_USERS_500]
			}

			postsCount, err4 := a.Srv().Store.Post().AnalyticsPostCount("", false, false)
			if err4 != nil {
				mlog.Debug("Error attempting to get number of posts.", mlog.Err(err4))
			}

			if postsCount > model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_POSTS_2M].Limit && warnMetricStatusFromStore[model.SYSTEM_WARN_METRIC_NUMBER_OF_POSTS_2M] != model.WARN_METRIC_STATUS_RUNONCE {
				tWarnMetric = model.WarnMetricsTable[model.SYSTEM_WARN_METRIC_NUMBER_OF_POSTS_2M]
			}

			if tWarnMetric != (model.WarnMetric{}) {
				warnMetrics = append(warnMetrics, tWarnMetric)
			}
		}
	}

	isE0Edition := model.BuildEnterpriseReady == "true" // license == nil was already validated upstream

	for _, warnMetric := range warnMetrics {
		data, nErr := a.Srv().Store.System().GetByName(warnMetric.Id)
		if nErr == nil && data != nil && warnMetric.IsBotOnly && data.Value == model.WARN_METRIC_STATUS_RUNONCE {
			mlog.Debug("This metric warning is bot only and ran once")
			continue
		}

		warnMetricStatus, _ := a.getWarnMetricStatusAndDisplayTextsForId(warnMetric.Id, nil, isE0Edition)
		if !warnMetric.IsBotOnly {
			// Banner and bot metric types - send websocket event every interval
			message := model.NewWebSocketEvent(model.WEBSOCKET_WARN_METRIC_STATUS_RECEIVED, "", "", "", nil)
			message.Add("warnMetricStatus", warnMetricStatus.ToJson())
			a.Publish(message)

			// Banner and bot metric types, send the bot message only once
			if data != nil && data.Value == model.WARN_METRIC_STATUS_RUNONCE {
				continue
			}
		}

		if nerr := a.notifyAdminsOfWarnMetricStatus(c, warnMetric.Id, isE0Edition); nerr != nil {
			mlog.Error("Failed to send notifications to admin users.", mlog.Err(nerr))
		}

		if warnMetric.IsRunOnce {
			a.setWarnMetricsStatusForId(warnMetric.Id, model.WARN_METRIC_STATUS_RUNONCE)
		} else {
			a.setWarnMetricsStatusForId(warnMetric.Id, model.WARN_METRIC_STATUS_LIMIT_REACHED)
		}
	}
}

func doCheckAdminSupportStatus(a *App, c *request.Context) {
	isE0Edition := model.BuildEnterpriseReady == "true"

	if strings.TrimSpace(*a.Config().SupportSettings.SupportEmail) == model.SUPPORT_SETTINGS_DEFAULT_SUPPORT_EMAIL {
		if err := a.notifyAdminsOfWarnMetricStatus(c, model.SYSTEM_METRIC_SUPPORT_EMAIL_NOT_CONFIGURED, isE0Edition); err != nil {
			mlog.Error("Failed to send notifications to admin users.", mlog.Err(err))
		}
	}
}

func (s *Server) StopMetricsServer() {
	s.metricsLock.Lock()
	defer s.metricsLock.Unlock()

	if s.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), TimeToWaitForConnectionsToCloseOnServerShutdown)
		defer cancel()

		s.metricsServer.Shutdown(ctx)
		s.Log.Info("Metrics and profiling server is stopping")
	}
}

func (s *Server) HandleMetrics(route string, h http.Handler) {
	if s.metricsRouter != nil {
		s.metricsRouter.Handle(route, h)
	}
}

func (s *Server) InitMetricsRouter() error {
	s.metricsRouter = mux.NewRouter()
	runtime.SetBlockProfileRate(*s.Config().MetricsSettings.BlockProfileRate)

	metricsPage := `
			<html>
				<body>{{if .}}
					<div><a href="/metrics">Metrics</a></div>{{end}}
					<div><a href="/debug/pprof/">Profiling Root</a></div>
					<div><a href="/debug/pprof/cmdline">Profiling Command Line</a></div>
					<div><a href="/debug/pprof/symbol">Profiling Symbols</a></div>
					<div><a href="/debug/pprof/goroutine">Profiling Goroutines</a></div>
					<div><a href="/debug/pprof/heap">Profiling Heap</a></div>
					<div><a href="/debug/pprof/threadcreate">Profiling Threads</a></div>
					<div><a href="/debug/pprof/block">Profiling Blocking</a></div>
					<div><a href="/debug/pprof/trace">Profiling Execution Trace</a></div>
					<div><a href="/debug/pprof/profile">Profiling CPU</a></div>
				</body>
			</html>
		`
	metricsPageTmpl, err := template.New("page").Parse(metricsPage)
	if err != nil {
		return errors.Wrap(err, "failed to create template")
	}

	rootHandler := func(w http.ResponseWriter, r *http.Request) {
		metricsPageTmpl.Execute(w, s.Metrics != nil)
	}

	s.metricsRouter.HandleFunc("/", rootHandler)
	s.metricsRouter.StrictSlash(true)

	s.metricsRouter.Handle("/debug", http.RedirectHandler("/", http.StatusMovedPermanently))
	s.metricsRouter.HandleFunc("/debug/pprof/", pprof.Index)
	s.metricsRouter.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.metricsRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.metricsRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.metricsRouter.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Manually add support for paths linked to by index page at /debug/pprof/
	s.metricsRouter.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	s.metricsRouter.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	s.metricsRouter.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	s.metricsRouter.Handle("/debug/pprof/block", pprof.Handler("block"))

	return nil
}

func (s *Server) startMetricsServer() {
	var notify chan struct{}
	s.metricsLock.Lock()
	defer func() {
		if notify != nil {
			<-notify
		}
		s.metricsLock.Unlock()
	}()

	l, err := net.Listen("tcp", *s.Config().MetricsSettings.ListenAddress)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	notify = make(chan struct{})
	s.metricsServer = &http.Server{
		Handler:      handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(s.metricsRouter),
		ReadTimeout:  time.Duration(*s.Config().ServiceSettings.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(*s.Config().ServiceSettings.WriteTimeout) * time.Second,
	}

	go func() {
		close(notify)
		if err := s.metricsServer.Serve(l); err != nil && err != http.ErrServerClosed {
			mlog.Critical(err.Error())
		}
	}()

	s.Log.Info("Metrics and profiling server is started", mlog.String("address", l.Addr().String()))
}

func (s *Server) sendLicenseUpForRenewalEmail(users map[string]*model.User, license *model.License) *model.AppError {
	key := model.LICENSE_UP_FOR_RENEWAL_EMAIL_SENT + license.Id
	if _, err := s.Store.System().GetByName(key); err == nil {
		// return early because the key already exists and that means we already executed the code below to send email successfully
		return nil
	}

	daysToExpiration := license.DaysToExpiration()

	renewalLink, appErr := s.GenerateLicenseRenewalLink()
	if appErr != nil {
		return model.NewAppError("s.sendLicenseUpForRenewalEmail", "api.server.license_up_for_renewal.error_generating_link", nil, appErr.Error(), http.StatusInternalServerError)
	}

	// we want to at least have one email sent out to an admin
	countNotOks := 0

	for _, user := range users {
		name := user.FirstName
		if name == "" {
			name = user.Username
		}
		ok, err := s.EmailService.SendLicenseUpForRenewalEmail(user.Email, name, user.Locale, *s.Config().ServiceSettings.SiteURL, renewalLink, daysToExpiration)
		if !ok || err != nil {
			mlog.Error("Error sending license up for renewal email to", mlog.String("user_email", user.Email))
			countNotOks++
		}
	}

	// if not even one admin got an email, we consider that this operation errored
	if countNotOks == len(users) {
		return model.NewAppError("s.sendLicenseUpForRenewalEmail", "api.server.license_up_for_renewal.error_sending_email", nil, "", http.StatusInternalServerError)
	}

	system := model.System{
		Name:  key,
		Value: "true",
	}

	if err := s.Store.System().Save(&system); err != nil {
		mlog.Debug("Failed to mark license up for renewal email sending as completed.", mlog.Err(err))
	}

	return nil
}

func (s *Server) doLicenseExpirationCheck() {
	s.LoadLicense()
	license := s.License()

	if license == nil {
		mlog.Debug("License cannot be found.")
		return
	}

	users, err := s.Store.User().GetSystemAdminProfiles()
	if err != nil {
		mlog.Error("Failed to get system admins for license expired message from Mattermost.")
		return
	}

	if license.IsWithinExpirationPeriod() {
		appErr := s.sendLicenseUpForRenewalEmail(users, license)
		if appErr != nil {
			mlog.Debug(appErr.Error())
		}
	}

	if !license.IsPastGracePeriod() {
		mlog.Debug("License is not past the grace period.")
		return
	}

	//send email to admin(s)
	for _, user := range users {
		user := user
		if user.Email == "" {
			mlog.Error("Invalid system admin email.", mlog.String("user_email", user.Email))
			continue
		}

		mlog.Debug("Sending license expired email.", mlog.String("user_email", user.Email))
		s.Go(func() {
			if err := s.EmailService.SendRemoveExpiredLicenseEmail(user.Email, user.Locale, *s.Config().ServiceSettings.SiteURL); err != nil {
				mlog.Error("Error while sending the license expired email.", mlog.String("user_email", user.Email), mlog.Err(err))
			}
		})
	}

	//remove the license
	s.RemoveLicense()
}

func (s *Server) StartSearchEngine() (string, string) {
	if s.SearchEngine.ElasticsearchEngine != nil && s.SearchEngine.ElasticsearchEngine.IsActive() {
		s.Go(func() {
			if err := s.SearchEngine.ElasticsearchEngine.Start(); err != nil {
				s.Log.Error(err.Error())
			}
		})
	}

	configListenerId := s.AddConfigListener(func(oldConfig *model.Config, newConfig *model.Config) {
		if s.SearchEngine == nil {
			return
		}
		s.SearchEngine.UpdateConfig(newConfig)

		if s.SearchEngine.ElasticsearchEngine != nil && !*oldConfig.ElasticsearchSettings.EnableIndexing && *newConfig.ElasticsearchSettings.EnableIndexing {
			s.Go(func() {
				if err := s.SearchEngine.ElasticsearchEngine.Start(); err != nil {
					mlog.Error(err.Error())
				}
			})
		} else if s.SearchEngine.ElasticsearchEngine != nil && *oldConfig.ElasticsearchSettings.EnableIndexing && !*newConfig.ElasticsearchSettings.EnableIndexing {
			s.Go(func() {
				if err := s.SearchEngine.ElasticsearchEngine.Stop(); err != nil {
					mlog.Error(err.Error())
				}
			})
		} else if s.SearchEngine.ElasticsearchEngine != nil && *oldConfig.ElasticsearchSettings.Password != *newConfig.ElasticsearchSettings.Password || *oldConfig.ElasticsearchSettings.Username != *newConfig.ElasticsearchSettings.Username || *oldConfig.ElasticsearchSettings.ConnectionUrl != *newConfig.ElasticsearchSettings.ConnectionUrl || *oldConfig.ElasticsearchSettings.Sniff != *newConfig.ElasticsearchSettings.Sniff {
			s.Go(func() {
				if *oldConfig.ElasticsearchSettings.EnableIndexing {
					if err := s.SearchEngine.ElasticsearchEngine.Stop(); err != nil {
						mlog.Error(err.Error())
					}
					if err := s.SearchEngine.ElasticsearchEngine.Start(); err != nil {
						mlog.Error(err.Error())
					}
				}
			})
		}
	})

	licenseListenerId := s.AddLicenseListener(func(oldLicense, newLicense *model.License) {
		if s.SearchEngine == nil {
			return
		}
		if oldLicense == nil && newLicense != nil {
			if s.SearchEngine.ElasticsearchEngine != nil && s.SearchEngine.ElasticsearchEngine.IsActive() {
				s.Go(func() {
					if err := s.SearchEngine.ElasticsearchEngine.Start(); err != nil {
						mlog.Error(err.Error())
					}
				})
			}
		} else if oldLicense != nil && newLicense == nil {
			if s.SearchEngine.ElasticsearchEngine != nil {
				s.Go(func() {
					if err := s.SearchEngine.ElasticsearchEngine.Stop(); err != nil {
						mlog.Error(err.Error())
					}
				})
			}
		}
	})

	return configListenerId, licenseListenerId
}

func (s *Server) stopSearchEngine() {
	s.RemoveConfigListener(s.searchConfigListenerId)
	s.RemoveLicenseListener(s.searchLicenseListenerId)
	if s.SearchEngine != nil && s.SearchEngine.ElasticsearchEngine != nil && s.SearchEngine.ElasticsearchEngine.IsActive() {
		s.SearchEngine.ElasticsearchEngine.Stop()
	}
	if s.SearchEngine != nil && s.SearchEngine.BleveEngine != nil && s.SearchEngine.BleveEngine.IsActive() {
		s.SearchEngine.BleveEngine.Stop()
	}
}

func (s *Server) FileBackend() (filestore.FileBackend, *model.AppError) {
	license := s.License()
	backend, err := filestore.NewFileBackend(s.Config().FileSettings.ToFileBackendSettings(license != nil && *license.Features.Compliance))
	if err != nil {
		return nil, model.NewAppError("FileBackend", "api.file.no_driver.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return backend, nil
}

func (s *Server) TotalWebsocketConnections() int {
	// This method is only called after the hub is initialized.
	// Therefore, no mutex is needed to protect s.hubs.
	count := int64(0)
	for _, hub := range s.hubs {
		count = count + atomic.LoadInt64(&hub.connectionCount)
	}

	return int(count)
}

func (s *Server) ClusterHealthScore() int {
	return s.Cluster.HealthScore()
}

func (s *Server) configOrLicenseListener() {
	s.regenerateClientConfig()
}

func (s *Server) ClientConfigHash() string {
	return s.clientConfigHash.Load().(string)
}

func (s *Server) initJobs() {
	s.Jobs = jobs.NewJobServer(s, s.Store, s.Metrics)
	if jobsDataRetentionJobInterface != nil {
		s.Jobs.DataRetentionJob = jobsDataRetentionJobInterface(s)
	}
	if jobsMessageExportJobInterface != nil {
		s.Jobs.MessageExportJob = jobsMessageExportJobInterface(s)
	}
	if jobsElasticsearchAggregatorInterface != nil {
		s.Jobs.ElasticsearchAggregator = jobsElasticsearchAggregatorInterface(s)
	}
	if jobsElasticsearchIndexerInterface != nil {
		s.Jobs.ElasticsearchIndexer = jobsElasticsearchIndexerInterface(s)
	}
	if jobsBleveIndexerInterface != nil {
		s.Jobs.BleveIndexer = jobsBleveIndexerInterface(s)
	}
	if jobsMigrationsInterface != nil {
		s.Jobs.Migrations = jobsMigrationsInterface(s)
	}
	if jobsLdapSyncInterface != nil {
		s.Jobs.LdapSync = jobsLdapSyncInterface(s)
	}
	if jobsPluginsInterface != nil {
		s.Jobs.Plugins = jobsPluginsInterface(s)
	}
	if jobsExpiryNotifyInterface != nil {
		s.Jobs.ExpiryNotify = jobsExpiryNotifyInterface(s)
	}
	if productNoticesJobInterface != nil {
		s.Jobs.ProductNotices = productNoticesJobInterface(s)
	}
	if jobsImportProcessInterface != nil {
		s.Jobs.ImportProcess = jobsImportProcessInterface(s)
	}
	if jobsImportDeleteInterface != nil {
		s.Jobs.ImportDelete = jobsImportDeleteInterface(s)
	}
	if jobsExportDeleteInterface != nil {
		s.Jobs.ExportDelete = jobsExportDeleteInterface(s)
	}

	if jobsExportProcessInterface != nil {
		s.Jobs.ExportProcess = jobsExportProcessInterface(s)
	}

	if jobsExportProcessInterface != nil {
		s.Jobs.ExportProcess = jobsExportProcessInterface(s)
	}

	if jobsActiveUsersInterface != nil {
		s.Jobs.ActiveUsers = jobsActiveUsersInterface(s)
	}

	if jobsCloudInterface != nil {
		s.Jobs.Cloud = jobsCloudInterface(s)
	}

	if jobsResendInvitationEmailInterface != nil {
		s.Jobs.ResendInvitationEmails = jobsResendInvitationEmailInterface(s)
	}

	s.Jobs.InitWorkers()
	s.Jobs.InitSchedulers()
}

func (s *Server) TelemetryId() string {
	if s.telemetryService == nil {
		return ""
	}
	return s.telemetryService.TelemetryID
}

func (s *Server) HttpService() httpservice.HTTPService {
	return s.HTTPService
}

func (s *Server) SetLog(l *mlog.Logger) {
	s.Log = l
}

func (s *Server) GetLogger() mlog.LoggerIFace {
	return s.Log
}

// GetStore returns the server's Store. Exposing via a method
// allows interfaces to be created with subsets of server APIs.
func (s *Server) GetStore() store.Store {
	return s.Store
}

// GetRemoteClusterService returns the `RemoteClusterService` instantiated by the server.
// May be nil if the service is not enabled via license.
func (s *Server) GetRemoteClusterService() remotecluster.RemoteClusterServiceIFace {
	s.serviceMux.RLock()
	defer s.serviceMux.RUnlock()
	return s.remoteClusterService
}

// GetSharedChannelSyncService returns the `SharedChannelSyncService` instantiated by the server.
// May be nil if the service is not enabled via license.
func (s *Server) GetSharedChannelSyncService() SharedChannelServiceIFace {
	s.serviceMux.RLock()
	defer s.serviceMux.RUnlock()
	return s.sharedChannelService
}

// GetMetrics returns the server's Metrics interface. Exposing via a method
// allows interfaces to be created with subsets of server APIs.
func (s *Server) GetMetrics() einterfaces.MetricsInterface {
	return s.Metrics
}

// SetRemoteClusterService sets the `RemoteClusterService` to be used by the server.
// For testing only.
func (s *Server) SetRemoteClusterService(remoteClusterService remotecluster.RemoteClusterServiceIFace) {
	s.serviceMux.Lock()
	defer s.serviceMux.Unlock()
	s.remoteClusterService = remoteClusterService
}

// SetSharedChannelSyncService sets the `SharedChannelSyncService` to be used by the server.
// For testing only.
func (s *Server) SetSharedChannelSyncService(sharedChannelService SharedChannelServiceIFace) {
	s.serviceMux.Lock()
	defer s.serviceMux.Unlock()
	s.sharedChannelService = sharedChannelService
}

func (a *App) GenerateSupportPacket() []model.FileData {
	// If any errors we come across within this function, we will log it in a warning.txt file so that we know why certain files did not get produced if any
	var warnings []string

	// Creating an array of files that we are going to be adding to our zip file
	fileDatas := []model.FileData{}

	// A array of the functions that we can iterate through since they all have the same return value
	functions := []func() (*model.FileData, string){
		a.generateSupportPacketYaml,
		a.createPluginsFile,
		a.createSanitizedConfigFile,
		a.getMattermostLog,
		a.getNotificationsLog,
	}

	for _, fn := range functions {
		fileData, warning := fn()

		if fileData != nil {
			fileDatas = append(fileDatas, *fileData)
		} else {
			warnings = append(warnings, warning)
		}
	}

	// Adding a warning.txt file to the fileDatas if any warning
	if len(warnings) > 0 {
		finalWarning := strings.Join(warnings, "\n")
		fileDatas = append(fileDatas, model.FileData{
			Filename: "warning.txt",
			Body:     []byte(finalWarning),
		})
	}

	return fileDatas
}

func (a *App) getNotificationsLog() (*model.FileData, string) {
	var warning string

	// Getting notifications.log
	if *a.Srv().Config().NotificationLogSettings.EnableFile {
		// notifications.log
		notificationsLog := utils.GetNotificationsLogFileLocation(*a.Srv().Config().LogSettings.FileLocation)

		notificationsLogFileData, notificationsLogFileDataErr := ioutil.ReadFile(notificationsLog)

		if notificationsLogFileDataErr == nil {
			fileData := model.FileData{
				Filename: "notifications.log",
				Body:     notificationsLogFileData,
			}
			return &fileData, ""
		}

		warning = fmt.Sprintf("ioutil.ReadFile(notificationsLog) Error: %s", notificationsLogFileDataErr.Error())

	} else {
		warning = "Unable to retrieve notifications.log because LogSettings: EnableFile is false in config.json"
	}

	return nil, warning
}

func (a *App) getMattermostLog() (*model.FileData, string) {
	var warning string

	// Getting mattermost.log
	if *a.Srv().Config().LogSettings.EnableFile {
		// mattermost.log
		mattermostLog := utils.GetLogFileLocation(*a.Srv().Config().LogSettings.FileLocation)

		mattermostLogFileData, mattermostLogFileDataErr := ioutil.ReadFile(mattermostLog)

		if mattermostLogFileDataErr == nil {
			fileData := model.FileData{
				Filename: "mattermost.log",
				Body:     mattermostLogFileData,
			}
			return &fileData, ""
		}
		warning = fmt.Sprintf("ioutil.ReadFile(mattermostLog) Error: %s", mattermostLogFileDataErr.Error())

	} else {
		warning = "Unable to retrieve mattermost.log because LogSettings: EnableFile is false in config.json"
	}

	return nil, warning
}

func (a *App) createSanitizedConfigFile() (*model.FileData, string) {
	// Getting sanitized config, prettifying it, and then adding it to our file data array
	sanitizedConfigPrettyJSON, err := json.MarshalIndent(a.GetSanitizedConfig(), "", "    ")
	if err == nil {
		fileData := model.FileData{
			Filename: "sanitized_config.json",
			Body:     sanitizedConfigPrettyJSON,
		}
		return &fileData, ""
	}

	warning := fmt.Sprintf("json.MarshalIndent(c.App.GetSanitizedConfig()) Error: %s", err.Error())
	return nil, warning
}

func (a *App) createPluginsFile() (*model.FileData, string) {
	var warning string

	// Getting the plugins installed on the server, prettify it, and then add them to the file data array
	pluginsResponse, appErr := a.GetPlugins()
	if appErr == nil {
		pluginsPrettyJSON, err := json.MarshalIndent(pluginsResponse, "", "    ")
		if err == nil {
			fileData := model.FileData{
				Filename: "plugins.json",
				Body:     pluginsPrettyJSON,
			}

			return &fileData, ""
		}

		warning = fmt.Sprintf("json.MarshalIndent(pluginsResponse) Error: %s", err.Error())
	} else {
		warning = fmt.Sprintf("c.App.GetPlugins() Error: %s", appErr.Error())
	}

	return nil, warning
}

func (a *App) generateSupportPacketYaml() (*model.FileData, string) {
	// Here we are getting information regarding Elastic Search
	var elasticServerVersion string
	var elasticServerPlugins []string
	if a.Srv().SearchEngine.ElasticsearchEngine != nil {
		elasticServerVersion = a.Srv().SearchEngine.ElasticsearchEngine.GetFullVersion()
		elasticServerPlugins = a.Srv().SearchEngine.ElasticsearchEngine.GetPlugins()
	}

	// Here we are getting information regarding LDAP
	ldapInterface := a.Srv().Ldap
	var vendorName, vendorVersion string
	if ldapInterface != nil {
		vendorName, vendorVersion = ldapInterface.GetVendorNameAndVendorVersion()
	}

	// Here we are getting information regarding the database (mysql/postgres + current Mattermost version)
	databaseType, databaseVersion := a.Srv().DatabaseTypeAndMattermostVersion()

	// Creating the struct for support packet yaml file
	supportPacket := model.SupportPacket{
		ServerOS:             runtime.GOOS,
		ServerArchitecture:   runtime.GOARCH,
		DatabaseType:         databaseType,
		DatabaseVersion:      databaseVersion,
		LdapVendorName:       vendorName,
		LdapVendorVersion:    vendorVersion,
		ElasticServerVersion: elasticServerVersion,
		ElasticServerPlugins: elasticServerPlugins,
	}

	// Marshal to a Yaml File
	supportPacketYaml, err := yaml.Marshal(&supportPacket)
	if err == nil {
		fileData := model.FileData{
			Filename: "support_packet.yaml",
			Body:     supportPacketYaml,
		}
		return &fileData, ""
	}

	warning := fmt.Sprintf("yaml.Marshal(&supportPacket) Error: %s", err.Error())
	return nil, warning
}

func (s *Server) GetProfileImage(user *model.User) ([]byte, bool, *model.AppError) {
	if *s.Config().FileSettings.DriverName == "" {
		img, appErr := s.GetDefaultProfileImage(user)
		if appErr != nil {
			return nil, false, appErr
		}
		return img, false, nil
	}

	path := "users/" + user.Id + "/profile.png"

	data, err := s.ReadFile(path)
	if err != nil {
		img, appErr := s.GetDefaultProfileImage(user)
		if appErr != nil {
			return nil, false, appErr
		}

		if user.LastPictureUpdate == 0 {
			if _, err := s.writeFile(bytes.NewReader(img), path); err != nil {
				return nil, false, err
			}
		}
		return img, true, nil
	}

	return data, false, nil
}

func (s *Server) GetDefaultProfileImage(user *model.User) ([]byte, *model.AppError) {
	img, err := s.userService.GetDefaultProfileImage(user)
	if err != nil {
		switch {
		case errors.Is(err, users.DefaultFontError):
			return nil, model.NewAppError("GetDefaultProfileImage", "api.user.create_profile_image.default_font.app_error", nil, err.Error(), http.StatusInternalServerError)
		case errors.Is(err, users.UserInitialsError):
			return nil, model.NewAppError("GetDefaultProfileImage", "api.user.create_profile_image.initial.app_error", nil, err.Error(), http.StatusInternalServerError)
		default:
			return nil, model.NewAppError("GetDefaultProfileImage", "api.user.create_profile_image.encode.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return img, nil
}

func (s *Server) ReadFile(path string) ([]byte, *model.AppError) {
	backend, err := s.FileBackend()
	if err != nil {
		return nil, err
	}
	result, nErr := backend.ReadFile(path)
	if nErr != nil {
		return nil, model.NewAppError("ReadFile", "api.file.read_file.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return result, nil
}

// func (s *Server) WriteFile(fr io.Reader, path string) (int64, *model.AppError) {
// 	backend, err := s.FileBackend()
// 	if err != nil {
// 		return 0, err
// 	}

// 	result, nErr := backend.WriteFile(fr, path)
// 	if nErr != nil {
// 		return result, model.NewAppError("WriteFile", "api.file.write_file.app_error", nil, nErr.Error(), http.StatusInternalServerError)
// 	}
// 	return result, nil
// }

func createDNDStatusExpirationRecurringTask(a *App) {
	a.srv.dndTaskMut.Lock()
	a.srv.dndTask = model.CreateRecurringTaskFromNextIntervalTime("Unset DND Statuses", a.UpdateDNDStatusOfUsers, 5*time.Minute)
	a.srv.dndTaskMut.Unlock()
}

func cancelDNDStatusExpirationRecurringTask(a *App) {
	a.srv.dndTaskMut.Lock()
	if a.srv.dndTask != nil {
		a.srv.dndTask.Cancel()
		a.srv.dndTask = nil
	}
	a.srv.dndTaskMut.Unlock()
}

func runDNDStatusExpireJob(a *App) {
	if !a.Config().FeatureFlags.TimedDND {
		return
	}
	if a.IsLeader() {
		createDNDStatusExpirationRecurringTask(a)
	}
	a.srv.AddClusterLeaderChangedListener(func() {
		mlog.Info("Cluster leader changed. Determining if unset DNS status task should be running", mlog.Bool("isLeader", a.IsLeader()))
		if a.IsLeader() {
			createDNDStatusExpirationRecurringTask(a)
		} else {
			cancelDNDStatusExpirationRecurringTask(a)
		}
	})
}

func stopDNDStatusExpireJob(a *App) {
	if a.IsLeader() {
		cancelDNDStatusExpirationRecurringTask(a)
	}
}
