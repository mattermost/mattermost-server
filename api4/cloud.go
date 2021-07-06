// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitCloud() {
	// GET /api/v4/cloud/products
	api.BaseRoutes.Cloud.Handle("/products", api.ApiSessionRequired(getCloudProducts)).Methods("GET")

	// POST /api/v4/cloud/payment
	// POST /api/v4/cloud/payment/confirm
	api.BaseRoutes.Cloud.Handle("/payment", api.ApiSessionRequired(createCustomerPayment)).Methods("POST")
	api.BaseRoutes.Cloud.Handle("/payment/confirm", api.ApiSessionRequired(confirmCustomerPayment)).Methods("POST")

	// GET /api/v4/cloud/customer
	// PUT /api/v4/cloud/customer
	// PUT /api/v4/cloud/customer/address
	api.BaseRoutes.Cloud.Handle("/customer", api.ApiSessionRequired(getCloudCustomer)).Methods("GET")
	api.BaseRoutes.Cloud.Handle("/customer", api.ApiSessionRequired(updateCloudCustomer)).Methods("PUT")
	api.BaseRoutes.Cloud.Handle("/customer/address", api.ApiSessionRequired(updateCloudCustomerAddress)).Methods("PUT")

	// GET /api/v4/cloud/subscription
	api.BaseRoutes.Cloud.Handle("/subscription", api.ApiSessionRequired(getSubscription)).Methods("GET")
	api.BaseRoutes.Cloud.Handle("/subscription/invoices", api.ApiSessionRequired(getInvoicesForSubscription)).Methods("GET")
	api.BaseRoutes.Cloud.Handle("/subscription/invoices/{invoice_id:in_[A-Za-z0-9]+}/pdf", api.ApiSessionRequired(getSubscriptionInvoicePDF)).Methods("GET")
	api.BaseRoutes.Cloud.Handle("/subscription/limitreached/invite", api.ApiSessionRequired(sendAdminUpgradeRequestEmail)).Methods("POST")
	api.BaseRoutes.Cloud.Handle("/subscription/limitreached/join", api.ApiHandler(sendAdminUpgradeRequestEmailOnJoin)).Methods("POST")
	api.BaseRoutes.Cloud.Handle("/subscription/stats", api.ApiHandler(getSubscriptionStats)).Methods("GET")
	api.BaseRoutes.Cloud.Handle("/subscription", api.ApiSessionRequired(changeSubscription)).Methods("PUT")

	// POST /api/v4/cloud/webhook
	api.BaseRoutes.Cloud.Handle("/webhook", api.CloudApiKeyRequired(handleCWSWebhook)).Methods("POST")
}

func getSubscription(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.getSubscription", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_BILLING)
		return
	}

	subscription, err := c.App.Cloud().GetSubscription(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getSubscription", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(subscription)
	if err != nil {
		c.Err = model.NewAppError("Api4.getSubscription", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(json)
}

func changeSubscription(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.changeSubscription", "api.cloud.license_error", nil, "", http.StatusInternalServerError)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_BILLING)
		return
	}

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.changeSubscription", "api.cloud.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	var subscriptionChange *model.SubscriptionChange
	if err = json.Unmarshal(bodyBytes, &subscriptionChange); err != nil {
		c.Err = model.NewAppError("Api4.changeSubscription", "api.cloud.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	currentSubscription, appErr := c.App.Cloud().GetSubscription(c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.changeSubscription", "api.cloud.app_error", nil, appErr.Error(), http.StatusInternalServerError)
		return
	}

	changedSub, err := c.App.Cloud().ChangeSubscription(c.AppContext.Session().UserId, currentSubscription.ID, subscriptionChange)
	if err != nil {
		c.Err = model.NewAppError("Api4.changeSubscription", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	json, err := json.Marshal(changedSub)
	if err != nil {
		c.Err = model.NewAppError("Api4.changeSubscription", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	w.Write(json)
}

func getSubscriptionStats(c *Context, w http.ResponseWriter, r *http.Request) {
	s, err := c.App.GetSubscriptionStats()
	if err != nil {
		c.Err = err
		return
	}

	stats, _ := json.Marshal(s)

	w.Write([]byte(string(stats)))
}

func getCloudProducts(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.getCloudProducts", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_BILLING)
		return
	}

	products, err := c.App.Cloud().GetCloudProducts(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getCloudProducts", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(products)
	if err != nil {
		c.Err = model.NewAppError("Api4.getCloudProducts", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(json)
}

func getCloudCustomer(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.getCloudCustomer", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_BILLING)
		return
	}

	customer, err := c.App.Cloud().GetCloudCustomer(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getCloudCustomer", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(customer)
	if err != nil {
		c.Err = model.NewAppError("Api4.getCloudCustomer", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(json)
}

func updateCloudCustomer(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.updateCloudCustomer", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_BILLING)
		return
	}

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomer", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	var customerInfo *model.CloudCustomerInfo
	if err = json.Unmarshal(bodyBytes, &customerInfo); err != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomer", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	customer, appErr := c.App.Cloud().UpdateCloudCustomer(c.AppContext.Session().UserId, customerInfo)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomer", "api.cloud.request_error", nil, appErr.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(customer)
	if err != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomer", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(json)
}

func updateCloudCustomerAddress(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.updateCloudCustomerAddress", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_BILLING)
		return
	}

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomerAddress", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	var address *model.Address
	if err = json.Unmarshal(bodyBytes, &address); err != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomerAddress", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	customer, appErr := c.App.Cloud().UpdateCloudCustomerAddress(c.AppContext.Session().UserId, address)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomerAddress", "api.cloud.request_error", nil, appErr.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(customer)
	if err != nil {
		c.Err = model.NewAppError("Api4.updateCloudCustomerAddress", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(json)
}

func createCustomerPayment(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.createCustomerPayment", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_BILLING)
		return
	}

	auditRec := c.MakeAuditRecord("createCustomerPayment", audit.Fail)
	defer c.LogAuditRec(auditRec)

	intent, err := c.App.Cloud().CreateCustomerPayment(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = model.NewAppError("Api4.createCustomerPayment", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(intent)
	if err != nil {
		c.Err = model.NewAppError("Api4.createCustomerPayment", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	auditRec.Success()

	w.Write(json)
}

func confirmCustomerPayment(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.confirmCustomerPayment", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_BILLING)
		return
	}

	auditRec := c.MakeAuditRecord("confirmCustomerPayment", audit.Fail)
	defer c.LogAuditRec(auditRec)

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.confirmCustomerPayment", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	var confirmRequest *model.ConfirmPaymentMethodRequest
	if err = json.Unmarshal(bodyBytes, &confirmRequest); err != nil {
		c.Err = model.NewAppError("Api4.confirmCustomerPayment", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	err = c.App.Cloud().ConfirmCustomerPayment(c.AppContext.Session().UserId, confirmRequest)
	if err != nil {
		c.Err = model.NewAppError("Api4.createCustomerPayment", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}

func getInvoicesForSubscription(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.getInvoicesForSubscription", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_BILLING)
		return
	}

	invoices, appErr := c.App.Cloud().GetInvoicesForSubscription(c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.getInvoicesForSubscription", "api.cloud.request_error", nil, appErr.Error(), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(invoices)
	if err != nil {
		c.Err = model.NewAppError("Api4.getInvoicesForSubscription", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(json)
}

func getSubscriptionInvoicePDF(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.getSuscriptionInvoicePDF", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireInvoiceId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_BILLING) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_BILLING)
		return
	}

	pdfData, filename, appErr := c.App.Cloud().GetInvoicePDF(c.AppContext.Session().UserId, c.Params.InvoiceId)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.getSuscriptionInvoicePDF", "api.cloud.request_error", nil, appErr.Error(), http.StatusInternalServerError)
		return
	}

	writeFileResponse(
		filename,
		"application/pdf",
		int64(binary.Size(pdfData)),
		time.Now(),
		*c.App.Config().ServiceSettings.WebserverMode,
		bytes.NewReader(pdfData),
		false,
		w,
		r,
	)
}

func handleCWSWebhook(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.handleCWSWebhook", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Err = model.NewAppError("Api4.handleCWSWebhook", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var event *model.CWSWebhookPayload
	if err = json.Unmarshal(bodyBytes, &event); err != nil {
		c.Err = model.NewAppError("Api4.handleCWSWebhook", "api.cloud.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	switch event.Event {
	case model.EventTypeFailedPayment:
		if nErr := c.App.SendPaymentFailedEmail(event.FailedPayment); nErr != nil {
			c.Err = nErr
			return
		}
	case model.EventTypeFailedPaymentNoCard:
		if nErr := c.App.SendNoCardPaymentFailedEmail(); nErr != nil {
			c.Err = nErr
			return
		}
	case model.EventTypeSendAdminWelcomeEmail:
		user, appErr := c.App.GetUserByUsername(event.CloudWorkspaceOwner.UserName)
		if appErr != nil {
			c.Err = model.NewAppError("Api4.handleCWSWebhook", appErr.Id, nil, appErr.Error(), appErr.StatusCode)
			return
		}

		teams, appErr := c.App.GetAllTeams()
		if appErr != nil {
			c.Err = model.NewAppError("Api4.handleCWSWebhook", appErr.Id, nil, appErr.Error(), appErr.StatusCode)
			return
		}

		team := teams[0]

		subscription, err := c.App.Cloud().GetSubscription(user.Id)
		if err != nil {
			c.Err = model.NewAppError("Api4.handleCWSWebhook", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := c.App.Srv().EmailService.SendCloudWelcomeEmail(user.Email, user.Locale, team.InviteId, subscription.GetWorkSpaceNameFromDNS(), subscription.DNS, *c.App.Config().ServiceSettings.SiteURL); err != nil {
			c.Err = model.NewAppError("SendCloudWelcomeEmail", "api.user.send_cloud_welcome_email.error", nil, err.Error(), http.StatusInternalServerError)
			return
		}
	case model.EventTypeTrialWillEnd:
		endTimeStamp := event.SubscriptionTrialEndUnixTimeStamp
		t := time.Unix(endTimeStamp, 0)
		trialEndDate := fmt.Sprintf("%s %d, %d", t.Month(), t.Day(), t.Year())

		if appErr := c.App.SendCloudTrialEndWarningEmail(trialEndDate, *c.App.Config().ServiceSettings.SiteURL); appErr != nil {
			c.Err = appErr
			return
		}
	case model.EventTypeTrialEnded:
		if appErr := c.App.SendCloudTrialEndedEmail(); appErr != nil {
			c.Err = appErr
			return
		}

	default:
		c.Err = model.NewAppError("Api4.handleCWSWebhook", "api.cloud.cws_webhook_event_missing_error", nil, "", http.StatusNotFound)
		return
	}

	ReturnStatusOK(w)
}

func sendAdminUpgradeRequestEmail(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmail", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	user, appErr := c.App.GetUser(c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmail", appErr.Id, nil, appErr.Error(), appErr.StatusCode)
		return
	}

	sub, err := c.App.Cloud().GetSubscription(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmail", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if appErr = c.App.SendAdminUpgradeRequestEmail(user.Username, sub, model.InviteLimitation); appErr != nil {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmail", appErr.Id, nil, appErr.Error(), appErr.StatusCode)
		return
	}

	ReturnStatusOK(w)
}

func sendAdminUpgradeRequestEmailOnJoin(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmailOnJoin", "api.cloud.license_error", nil, "", http.StatusNotImplemented)
		return
	}

	sub, err := c.App.Cloud().GetSubscription("")
	if err != nil {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmailOnJoin", "api.cloud.request_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if appErr := c.App.SendAdminUpgradeRequestEmail("", sub, model.JoinLimitation); appErr != nil {
		c.Err = model.NewAppError("Api4.sendAdminUpgradeRequestEmail", appErr.Id, nil, appErr.Error(), appErr.StatusCode)
		return
	}

	ReturnStatusOK(w)
}
