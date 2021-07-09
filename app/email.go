// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/shared/templates"
)

const (
	emailRateLimitingMemstoreSize = 65536
	emailRateLimitingPerHour      = 20
	emailRateLimitingMaxBurst     = 20
)

func condenseSiteURL(siteURL string) string {
	parsedSiteURL, _ := url.Parse(siteURL)
	if parsedSiteURL.Path == "" || parsedSiteURL.Path == "/" {
		return parsedSiteURL.Host
	}

	return path.Join(parsedSiteURL.Host, parsedSiteURL.Path)
}

type EmailService struct {
	srv                     *Server
	PerHourEmailRateLimiter *throttled.GCRARateLimiter
	PerDayEmailRateLimiter  *throttled.GCRARateLimiter
	EmailBatching           *EmailBatchingJob
}

func NewEmailService(srv *Server) (*EmailService, error) {
	service := &EmailService{srv: srv}
	if err := service.setUpRateLimiters(); err != nil {
		return nil, err
	}
	service.InitEmailBatching()
	return service, nil
}

func (es *EmailService) setUpRateLimiters() error {
	store, err := memstore.New(emailRateLimitingMemstoreSize)
	if err != nil {
		return errors.Wrap(err, "Unable to setup email rate limiting memstore.")
	}

	perHourQuota := throttled.RateQuota{
		MaxRate:  throttled.PerHour(emailRateLimitingPerHour),
		MaxBurst: emailRateLimitingMaxBurst,
	}

	perDayQuota := throttled.RateQuota{
		MaxRate:  throttled.PerDay(1),
		MaxBurst: 0,
	}

	perHourRateLimiter, err := throttled.NewGCRARateLimiter(store, perHourQuota)
	if err != nil || perHourRateLimiter == nil {
		return errors.Wrap(err, "Unable to setup email rate limiting GCRA rate limiter.")
	}

	perDayRateLimiter, err := throttled.NewGCRARateLimiter(store, perDayQuota)
	if err != nil || perDayRateLimiter == nil {
		return errors.Wrap(err, "Unable to setup per day email rate limiting GCRA rate limiter.")
	}

	es.PerHourEmailRateLimiter = perHourRateLimiter
	es.PerDayEmailRateLimiter = perDayRateLimiter
	return nil
}

func (es *EmailService) sendChangeUsernameEmail(newUsername, email, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.username_change_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName,
			"TeamDisplayName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.username_change_body.title")
	data.Props["Info"] = T("api.templates.username_change_body.info",
		map[string]interface{}{"TeamDisplayName": es.srv.Config().TeamSettings.SiteName, "NewUsername": newUsername})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.srv.TemplatesContainer().RenderToString("email_change_body", data)
	if err != nil {
		return model.NewAppError("sendChangeUsernameEmail", "api.user.send_email_change_username_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("sendChangeUsernameEmail", "api.user.send_email_change_username_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendEmailChangeVerifyEmail(newUserEmail, locale, siteURL, token string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	link := fmt.Sprintf("%s/do_verify_email?token=%s&email=%s", siteURL, token, url.QueryEscape(newUserEmail))

	subject := T("api.templates.email_change_verify_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName,
			"TeamDisplayName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.email_change_verify_body.title")
	data.Props["Info"] = T("api.templates.email_change_verify_body.info",
		map[string]interface{}{"TeamDisplayName": es.srv.Config().TeamSettings.SiteName})
	data.Props["VerifyUrl"] = link
	data.Props["VerifyButton"] = T("api.templates.email_change_verify_body.button")

	body, err := es.srv.TemplatesContainer().RenderToString("email_change_verify_body", data)
	if err != nil {
		return model.NewAppError("sendEmailChangeVerifyEmail", "api.user.send_email_change_verify_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(newUserEmail, subject, body); err != nil {
		return model.NewAppError("sendEmailChangeVerifyEmail", "api.user.send_email_change_verify_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendEmailChangeEmail(oldEmail, newEmail, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.email_change_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName,
			"TeamDisplayName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.email_change_body.title")
	data.Props["Info"] = T("api.templates.email_change_body.info",
		map[string]interface{}{"TeamDisplayName": es.srv.Config().TeamSettings.SiteName, "NewEmail": newEmail})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.srv.TemplatesContainer().RenderToString("email_change_body", data)
	if err != nil {
		return model.NewAppError("sendEmailChangeEmail", "api.user.send_email_change_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(oldEmail, subject, body); err != nil {
		return model.NewAppError("sendEmailChangeEmail", "api.user.send_email_change_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendVerifyEmail(userEmail, locale, siteURL, token, redirect string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	link := fmt.Sprintf("%s/do_verify_email?token=%s&email=%s", siteURL, token, url.QueryEscape(userEmail))
	if redirect != "" {
		link += fmt.Sprintf("&redirect_to=%s", redirect)
	}

	serverURL := condenseSiteURL(siteURL)

	subject := T("api.templates.verify_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.verify_body.title")
	data.Props["SubTitle1"] = T("api.templates.verify_body.subTitle1")
	data.Props["ServerURL"] = T("api.templates.verify_body.serverURL", map[string]interface{}{"ServerURL": serverURL})
	data.Props["SubTitle2"] = T("api.templates.verify_body.subTitle2")
	data.Props["ButtonURL"] = link
	data.Props["Button"] = T("api.templates.verify_body.button")
	data.Props["Info"] = T("api.templates.verify_body.info")
	data.Props["Info1"] = T("api.templates.verify_body.info1")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.srv.TemplatesContainer().RenderToString("verify_body", data)
	if err != nil {
		return model.NewAppError("SendVerifyEmail", "api.user.send_verify_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return model.NewAppError("SendVerifyEmail", "api.user.send_verify_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) SendSignInChangeEmail(email, method, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.signin_change_email.subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.signin_change_email.body.title")
	data.Props["Info"] = T("api.templates.signin_change_email.body.info",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName, "Method": method})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.srv.TemplatesContainer().RenderToString("signin_change_body", data)
	if err != nil {
		return model.NewAppError("SendSignInChangeEmail", "api.user.send_sign_in_change_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("SendSignInChangeEmail", "api.user.send_sign_in_change_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendWelcomeEmail(userID string, email string, verified bool, disableWelcomeEmail bool, locale, siteURL, redirect string) *model.AppError {
	if disableWelcomeEmail {
		return nil
	}
	if !*es.srv.Config().EmailSettings.SendEmailNotifications && !*es.srv.Config().EmailSettings.RequireEmailVerification {
		return model.NewAppError("SendWelcomeEmail", "api.user.send_welcome_email_and_forget.failed.error", nil, "Send Email Notifications and Require Email Verification is disabled in the system console", http.StatusInternalServerError)
	}

	T := i18n.GetUserTranslations(locale)

	serverURL := condenseSiteURL(siteURL)

	subject := T("api.templates.welcome_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName,
			"ServerURL": serverURL})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.welcome_body.title")
	data.Props["SubTitle1"] = T("api.templates.welcome_body.subTitle1")
	data.Props["ServerURL"] = T("api.templates.welcome_body.serverURL", map[string]interface{}{"ServerURL": serverURL})
	data.Props["SubTitle2"] = T("api.templates.welcome_body.subTitle2")
	data.Props["Button"] = T("api.templates.welcome_body.button")
	data.Props["Info"] = T("api.templates.welcome_body.info")
	data.Props["Info1"] = T("api.templates.welcome_body.info1")
	data.Props["SiteURL"] = siteURL

	if *es.srv.Config().NativeAppSettings.AppDownloadLink != "" {
		data.Props["AppDownloadTitle"] = T("api.templates.welcome_body.app_download_title")
		data.Props["AppDownloadInfo"] = T("api.templates.welcome_body.app_download_info")
		data.Props["AppDownloadButton"] = T("api.templates.welcome_body.app_download_button")
		data.Props["AppDownloadLink"] = *es.srv.Config().NativeAppSettings.AppDownloadLink
	}

	if !verified && *es.srv.Config().EmailSettings.RequireEmailVerification {
		token, err := es.CreateVerifyEmailToken(userID, email)
		if err != nil {
			return err
		}
		link := fmt.Sprintf("%s/do_verify_email?token=%s&email=%s", siteURL, token.Token, url.QueryEscape(email))
		if redirect != "" {
			link += fmt.Sprintf("&redirect_to=%s", redirect)
		}
		data.Props["ButtonURL"] = link
	}

	body, err := es.srv.TemplatesContainer().RenderToString("welcome_body", data)
	if err != nil {
		return model.NewAppError("sendWelcomeEmail", "api.user.send_welcome_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("sendWelcomeEmail", "api.user.send_welcome_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) SendCloudTrialEndWarningEmail(userEmail, name, trialEndDate, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.cloud_trial_ending_email.subject")

	data := es.newEmailTemplateData(locale)
	data.Props["Title"] = T("api.templates.cloud_trial_ending_email.title")
	data.Props["SubTitle"] = T("api.templates.cloud_trial_ending_email.subtitle", map[string]interface{}{"Name": name, "TrialEnd": trialEndDate})
	data.Props["SiteURL"] = siteURL
	data.Props["ButtonURL"] = fmt.Sprintf("%s/admin_console/billing/subscription?action=show_purchase_modal", siteURL)
	data.Props["Button"] = T("api.templates.cloud_trial_ending_email.add_payment_method")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.srv.TemplatesContainer().RenderToString("cloud_trial_end_warning", data)
	if err != nil {
		return model.NewAppError("SendCloudTrialEndWarningEmail", "api.user.cloud_trial_ending_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return model.NewAppError("SendCloudTrialEndWarningEmail", "api.user.cloud_trial_ending_email.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (es *EmailService) SendCloudTrialEndedEmail(userEmail, name, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.cloud_trial_ended_email.subject")

	t := time.Now()
	todayDate := fmt.Sprintf("%s %d, %d", t.Month(), t.Day(), t.Year())

	data := es.newEmailTemplateData(locale)
	data.Props["Title"] = T("api.templates.cloud_trial_ended_email.title")
	data.Props["SubTitle"] = T("api.templates.cloud_trial_ended_email.subtitle", map[string]interface{}{"Name": name, "TodayDate": todayDate})
	data.Props["SiteURL"] = siteURL
	data.Props["ButtonURL"] = fmt.Sprintf("%s/admin_console/billing/subscription", siteURL)
	data.Props["Button"] = T("api.templates.cloud_trial_ended_email.start_subscription")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.srv.TemplatesContainer().RenderToString("cloud_trial_ended_email", data)
	if err != nil {
		return model.NewAppError("SendCloudTrialEndedEmail", "api.user.cloud_trial_ended_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return model.NewAppError("SendCloudTrialEndedEmail", "api.user.cloud_trial_ended_email.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

// SendCloudWelcomeEmail sends the cloud version of the welcome email
func (es *EmailService) SendCloudWelcomeEmail(userEmail, locale, teamInviteID, workSpaceName, dns, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.cloud_welcome_email.subject")

	data := es.newEmailTemplateData(locale)
	data.Props["Title"] = T("api.templates.cloud_welcome_email.title", map[string]interface{}{"WorkSpace": workSpaceName})
	data.Props["SubTitle"] = T("api.templates.cloud_welcome_email.subtitle")
	data.Props["SubTitleInfo"] = T("api.templates.cloud_welcome_email.subtitle_info")
	data.Props["Info"] = T("api.templates.cloud_welcome_email.info")
	data.Props["Info2"] = T("api.templates.cloud_welcome_email.info2")
	data.Props["WorkSpacePath"] = siteURL
	data.Props["DNS"] = dns
	data.Props["InviteInfo"] = T("api.templates.cloud_welcome_email.invite_info")
	data.Props["InviteSubInfo"] = T("api.templates.cloud_welcome_email.invite_sub_info", map[string]interface{}{"WorkSpace": workSpaceName})
	data.Props["InviteSubInfoLink"] = fmt.Sprintf("%s/signup_user_complete/?id=%s", siteURL, teamInviteID)
	data.Props["AddAppsInfo"] = T("api.templates.cloud_welcome_email.add_apps_info")
	data.Props["AddAppsSubInfo"] = T("api.templates.cloud_welcome_email.add_apps_sub_info")
	data.Props["AppMarketPlace"] = T("api.templates.cloud_welcome_email.app_market_place")
	data.Props["AppMarketPlaceLink"] = "https://integrations.mattermost.com/"
	data.Props["DownloadMMInfo"] = T("api.templates.cloud_welcome_email.download_mm_info")
	data.Props["SignInSubInfo"] = T("api.templates.cloud_welcome_email.signin_sub_info")
	data.Props["MMApps"] = T("api.templates.cloud_welcome_email.mm_apps")
	data.Props["SignInSubInfo2"] = T("api.templates.cloud_welcome_email.signin_sub_info2")
	data.Props["DownloadMMAppsLink"] = "https://mattermost.com/download/"
	data.Props["Button"] = T("api.templates.cloud_welcome_email.button")
	data.Props["GettingStartedQuestions"] = T("api.templates.cloud_welcome_email.start_questions")

	body, err := es.srv.TemplatesContainer().RenderToString("cloud_welcome_email", data)
	if err != nil {
		return model.NewAppError("SendCloudWelcomeEmail", "api.user.send_cloud_welcome_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return model.NewAppError("SendCloudWelcomeEmail", "api.user.send_cloud_welcome_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendPasswordChangeEmail(email, method, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.password_change_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName,
			"TeamDisplayName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.password_change_body.title")
	data.Props["Info"] = T("api.templates.password_change_body.info",
		map[string]interface{}{"TeamDisplayName": es.srv.Config().TeamSettings.SiteName, "TeamURL": siteURL, "Method": method})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.srv.TemplatesContainer().RenderToString("password_change_body", data)
	if err != nil {
		return model.NewAppError("sendPasswordChangeEmail", "api.user.send_password_change_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("sendPasswordChangeEmail", "api.user.send_password_change_email_and_forget.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendUserAccessTokenAddedEmail(email, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.user_access_token_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.user_access_token_body.title")
	data.Props["Info"] = T("api.templates.user_access_token_body.info",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName, "SiteURL": siteURL})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.srv.TemplatesContainer().RenderToString("password_change_body", data)
	if err != nil {
		return model.NewAppError("sendUserAccessTokenAddedEmail", "api.user.send_user_access_token.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("sendUserAccessTokenAddedEmail", "api.user.send_user_access_token.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) SendPasswordResetEmail(email string, token *model.Token, locale, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	link := fmt.Sprintf("%s/reset_password_complete?token=%s", siteURL, url.QueryEscape(token.Token))

	subject := T("api.templates.reset_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.reset_body.title")
	data.Props["SubTitle"] = T("api.templates.reset_body.subTitle")
	data.Props["Info"] = T("api.templates.reset_body.info")
	data.Props["ButtonURL"] = link
	data.Props["Button"] = T("api.templates.reset_body.button")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.srv.TemplatesContainer().RenderToString("reset_body", data)
	if err != nil {
		return false, model.NewAppError("SendPasswordReset", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendPasswordReset", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) sendMfaChangeEmail(email string, activated bool, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.mfa_change_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL

	if activated {
		data.Props["Info"] = T("api.templates.mfa_activated_body.info", map[string]interface{}{"SiteURL": siteURL})
		data.Props["Title"] = T("api.templates.mfa_activated_body.title")
	} else {
		data.Props["Info"] = T("api.templates.mfa_deactivated_body.info", map[string]interface{}{"SiteURL": siteURL})
		data.Props["Title"] = T("api.templates.mfa_deactivated_body.title")
	}
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.srv.TemplatesContainer().RenderToString("mfa_change_body", data)
	if err != nil {
		return model.NewAppError("SendMfaChangeEmail", "api.user.send_mfa_change_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("SendMfaChangeEmail", "api.user.send_mfa_change_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) SendInviteEmails(team *model.Team, senderName string, senderUserID string, invites []string, siteURL string) *model.AppError {
	if es.PerHourEmailRateLimiter == nil {
		return model.NewAppError("SendInviteEmails", "app.email.no_rate_limiter.app_error", nil, fmt.Sprintf("user_id=%s, team_id=%s", senderUserID, team.ID), http.StatusInternalServerError)
	}
	rateLimited, result, err := es.PerHourEmailRateLimiter.RateLimit(senderUserID, len(invites))
	if err != nil {
		return model.NewAppError("SendInviteEmails", "app.email.setup_rate_limiter.app_error", nil, fmt.Sprintf("user_id=%s, team_id=%s, error=%v", senderUserID, team.ID, err), http.StatusInternalServerError)
	}

	if rateLimited {
		return model.NewAppError("SendInviteEmails",
			"app.email.rate_limit_exceeded.app_error", map[string]interface{}{"RetryAfter": result.RetryAfter.String(), "ResetAfter": result.ResetAfter.String()},
			fmt.Sprintf("user_id=%s, team_id=%s, retry_after_secs=%f, reset_after_secs=%f",
				senderUserID, team.ID, result.RetryAfter.Seconds(), result.ResetAfter.Seconds()),
			http.StatusRequestEntityTooLarge)
	}

	for _, invite := range invites {
		if invite != "" {
			subject := i18n.T("api.templates.invite_subject",
				map[string]interface{}{"SenderName": senderName,
					"TeamDisplayName": team.DisplayName,
					"SiteName":        es.srv.Config().TeamSettings.SiteName})

			data := es.newEmailTemplateData("")
			data.Props["SiteURL"] = siteURL
			data.Props["Title"] = i18n.T("api.templates.invite_body.title", map[string]interface{}{"SenderName": senderName, "TeamDisplayName": team.DisplayName})
			data.Props["SubTitle"] = i18n.T("api.templates.invite_body.subTitle")
			data.Props["Button"] = i18n.T("api.templates.invite_body.button")
			data.Props["SenderName"] = senderName
			data.Props["InviteFooterTitle"] = i18n.T("api.templates.invite_body_footer.title")
			data.Props["InviteFooterInfo"] = i18n.T("api.templates.invite_body_footer.info")
			data.Props["InviteFooterLearnMore"] = i18n.T("api.templates.invite_body_footer.learn_more")

			token := model.NewToken(
				TokenTypeTeamInvitation,
				model.MapToJSON(map[string]string{"teamId": team.ID, "email": invite}),
			)

			tokenProps := make(map[string]string)
			tokenProps["email"] = invite
			tokenProps["display_name"] = team.DisplayName
			tokenProps["name"] = team.Name
			tokenData := model.MapToJSON(tokenProps)

			if err := es.srv.Store.Token().Save(token); err != nil {
				mlog.Error("Failed to send invite email successfully ", mlog.Err(err))
				continue
			}
			data.Props["ButtonURL"] = fmt.Sprintf("%s/signup_user_complete/?d=%s&t=%s", siteURL, url.QueryEscape(tokenData), url.QueryEscape(token.Token))

			body, err := es.srv.TemplatesContainer().RenderToString("invite_body", data)
			if err != nil {
				mlog.Error("Failed to send invite email successfully ", mlog.Err(err))
			}

			if err := es.sendMail(invite, subject, body); err != nil {
				mlog.Error("Failed to send invite email successfully ", mlog.Err(err))
			}
		}
	}
	return nil
}

func (es *EmailService) sendGuestInviteEmails(team *model.Team, channels []*model.Channel, senderName string, senderUserID string, senderProfileImage []byte, invites []string, siteURL string, message string) *model.AppError {
	if es.PerHourEmailRateLimiter == nil {
		return model.NewAppError("SendInviteEmails", "app.email.no_rate_limiter.app_error", nil, fmt.Sprintf("user_id=%s, team_id=%s", senderUserID, team.ID), http.StatusInternalServerError)
	}
	rateLimited, result, err := es.PerHourEmailRateLimiter.RateLimit(senderUserID, len(invites))
	if err != nil {
		return model.NewAppError("SendInviteEmails", "app.email.setup_rate_limiter.app_error", nil, fmt.Sprintf("user_id=%s, team_id=%s, error=%v", senderUserID, team.ID, err), http.StatusInternalServerError)
	}

	if rateLimited {
		return model.NewAppError("SendInviteEmails",
			"app.email.rate_limit_exceeded.app_error", map[string]interface{}{"RetryAfter": result.RetryAfter.String(), "ResetAfter": result.ResetAfter.String()},
			fmt.Sprintf("user_id=%s, team_id=%s, retry_after_secs=%f, reset_after_secs=%f",
				senderUserID, team.ID, result.RetryAfter.Seconds(), result.ResetAfter.Seconds()),
			http.StatusRequestEntityTooLarge)
	}

	for _, invite := range invites {
		if invite != "" {
			subject := i18n.T("api.templates.invite_guest_subject",
				map[string]interface{}{"SenderName": senderName,
					"TeamDisplayName": team.DisplayName,
					"SiteName":        es.srv.Config().TeamSettings.SiteName})

			data := es.newEmailTemplateData("")
			data.Props["SiteURL"] = siteURL
			data.Props["Title"] = i18n.T("api.templates.invite_body.title", map[string]interface{}{"SenderName": senderName, "TeamDisplayName": team.DisplayName})
			data.Props["SubTitle"] = i18n.T("api.templates.invite_body_guest.subTitle")
			data.Props["Button"] = i18n.T("api.templates.invite_body.button")
			data.Props["SenderName"] = senderName
			data.Props["Message"] = ""
			if message != "" {
				data.Props["Message"] = message
			}
			data.Props["InviteFooterTitle"] = i18n.T("api.templates.invite_body_footer.title")
			data.Props["InviteFooterInfo"] = i18n.T("api.templates.invite_body_footer.info")
			data.Props["InviteFooterLearnMore"] = i18n.T("api.templates.invite_body_footer.learn_more")

			channelIDs := []string{}
			for _, channel := range channels {
				channelIDs = append(channelIDs, channel.ID)
			}

			token := model.NewToken(
				TokenTypeGuestInvitation,
				model.MapToJSON(map[string]string{
					"teamId":   team.ID,
					"channels": strings.Join(channelIDs, " "),
					"email":    invite,
					"guest":    "true",
				}),
			)

			tokenProps := make(map[string]string)
			tokenProps["email"] = invite
			tokenProps["display_name"] = team.DisplayName
			tokenProps["name"] = team.Name
			tokenData := model.MapToJSON(tokenProps)

			if err := es.srv.Store.Token().Save(token); err != nil {
				mlog.Error("Failed to send invite email successfully ", mlog.Err(err))
				continue
			}
			data.Props["ButtonURL"] = fmt.Sprintf("%s/signup_user_complete/?d=%s&t=%s", siteURL, url.QueryEscape(tokenData), url.QueryEscape(token.Token))

			if !*es.srv.Config().EmailSettings.SendEmailNotifications {
				mlog.Info("sending invitation ", mlog.String("to", invite), mlog.String("link", data.Props["ButtomURL"].(string)))
			}

			embeddedFiles := make(map[string]io.Reader)
			if message != "" {
				if senderProfileImage != nil {
					data.Props["SenderPhoto"] = "user-avatar.png"
					embeddedFiles = map[string]io.Reader{
						"user-avatar.png": bytes.NewReader(senderProfileImage),
					}
				}
			}

			body, err := es.srv.TemplatesContainer().RenderToString("invite_body", data)
			if err != nil {
				mlog.Error("Failed to send invite email successfully", mlog.Err(err))
			}

			if nErr := es.sendMailWithEmbeddedFiles(invite, subject, body, embeddedFiles); nErr != nil {
				mlog.Error("Failed to send invite email successfully", mlog.Err(nErr))
			}
		}
	}
	return nil
}

func (es *EmailService) newEmailTemplateData(locale string) templates.Data {
	var localT i18n.TranslateFunc
	if locale != "" {
		localT = i18n.GetUserTranslations(locale)
	} else {
		localT = i18n.T
	}
	organization := ""

	if *es.srv.Config().EmailSettings.FeedbackOrganization != "" {
		organization = localT("api.templates.email_organization") + *es.srv.Config().EmailSettings.FeedbackOrganization
	}

	return templates.Data{
		Props: map[string]interface{}{
			"EmailInfo1": localT("api.templates.email_info1"),
			"EmailInfo2": localT("api.templates.email_info2"),
			"EmailInfo3": localT("api.templates.email_info3",
				map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName}),
			"SupportEmail": *es.srv.Config().SupportSettings.SupportEmail,
			"Footer":       localT("api.templates.email_footer"),
			"FooterV2":     localT("api.templates.email_footer_v2"),
			"Organization": organization,
		},
		HTML: map[string]template.HTML{},
	}
}

func (es *EmailService) SendDeactivateAccountEmail(email string, locale, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	serverURL := condenseSiteURL(siteURL)

	subject := T("api.templates.deactivate_subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName,
			"ServerURL": serverURL})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.deactivate_body.title", map[string]interface{}{"ServerURL": serverURL})
	data.Props["Info"] = T("api.templates.deactivate_body.info",
		map[string]interface{}{"SiteURL": siteURL})
	data.Props["Warning"] = T("api.templates.deactivate_body.warning")

	body, err := es.srv.TemplatesContainer().RenderToString("deactivate_body", data)
	if err != nil {
		return model.NewAppError("SendDeactivateEmail", "api.user.send_deactivate_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("SendDeactivateEmail", "api.user.send_deactivate_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

// SendRemoveExpiredLicenseEmail formats an email and uses the email service to send the email to user with link pointing to CWS
// to renew the user license
func (es *EmailService) SendRemoveExpiredLicenseEmail(email string, locale, siteURL string) *model.AppError {
	renewalLink, err := es.srv.GenerateLicenseRenewalLink()
	if err != nil {
		return err
	}

	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.remove_expired_license.subject",
		map[string]interface{}{"SiteName": es.srv.Config().TeamSettings.SiteName})

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.remove_expired_license.body.title")
	data.Props["Link"] = renewalLink
	data.Props["LinkButton"] = T("api.templates.remove_expired_license.body.renew_button")

	body, nErr := es.srv.TemplatesContainer().RenderToString("remove_expired_license", data)
	if nErr != nil {
		return model.NewAppError("SendRemoveExpiredLicenseEmail", "api.license.remove_expired_license.failed.error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("SendRemoveExpiredLicenseEmail", "api.license.remove_expired_license.failed.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (es *EmailService) sendNotificationMail(to, subject, htmlBody string) error {
	if !*es.srv.Config().EmailSettings.SendEmailNotifications {
		return nil
	}
	return es.sendMail(to, subject, htmlBody)
}

func (es *EmailService) sendMail(to, subject, htmlBody string) error {
	return es.sendMailWithCC(to, subject, htmlBody, "")
}

func (es *EmailService) sendMailWithCC(to, subject, htmlBody string, ccMail string) error {
	license := es.srv.License()
	mailConfig := es.srv.MailServiceConfig()

	return mail.SendMailUsingConfig(to, subject, htmlBody, mailConfig, license != nil && *license.Features.Compliance, ccMail)
}

func (es *EmailService) sendMailWithEmbeddedFiles(to, subject, htmlBody string, embeddedFiles map[string]io.Reader) error {
	license := es.srv.License()
	mailConfig := es.srv.MailServiceConfig()

	return mail.SendMailWithEmbeddedFilesUsingConfig(to, subject, htmlBody, embeddedFiles, mailConfig, license != nil && *license.Features.Compliance, "")
}

func (es *EmailService) CreateVerifyEmailToken(userID string, newEmail string) (*model.Token, *model.AppError) {
	tokenExtra := struct {
		UserID string
		Email  string
	}{
		userID,
		newEmail,
	}
	jsonData, err := json.Marshal(tokenExtra)

	if err != nil {
		return nil, model.NewAppError("CreateVerifyEmailToken", "api.user.create_email_token.error", nil, "", http.StatusInternalServerError)
	}

	token := model.NewToken(TokenTypeVerifyEmail, string(jsonData))

	if err = es.srv.Store.Token().Save(token); err != nil {
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("CreateVerifyEmailToken", "app.recover.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return token, nil
}

func (es *EmailService) SendAtUserLimitWarningEmail(email string, locale string, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.at_limit_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.at_limit_title")
	data.Props["Info1"] = T("api.templates.at_limit_info1")
	data.Props["Info2"] = T("api.templates.at_limit_info2")
	data.Props["Button"] = T("api.templates.upgrade_mattermost_cloud")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("reached_user_limit_body", data)
	if err != nil {
		return false, model.NewAppError("SendAtUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendAtUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendLicenseUpForRenewalEmail(email, name, locale, siteURL, renewalLink string, daysToExpiration int) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.license_up_for_renewal_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.license_up_for_renewal_title")
	data.Props["SubTitle"] = T("api.templates.license_up_for_renewal_subtitle", map[string]interface{}{"UserName": name, "Days": daysToExpiration})
	data.Props["SubTitleTwo"] = T("api.templates.license_up_for_renewal_subtitle_two")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")
	data.Props["Button"] = T("api.templates.license_up_for_renewal_renew_now")
	data.Props["ButtonURL"] = renewalLink
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.srv.TemplatesContainer().RenderToString("license_up_for_renewal", data)
	if err != nil {
		return false, model.NewAppError("SendLicenseUpForRenewalEmail", "api.user.send_license_up_for_renewal_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendLicenseUpForRenewalEmail", "api.user.send_license_up_for_renewal_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

// SendUpgradeEmail formats an email template and sends an email to an admin specified in the email arg
func (es *EmailService) SendUpgradeEmail(user, email, locale, siteURL, action string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.upgrade_request_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["Info5"] = T("api.templates.at_limit_info5")
	data.Props["BillingPath"] = "admin_console/billing/subscription"
	data.Props["SiteURL"] = siteURL
	data.Props["Button"] = T("api.templates.upgrade_mattermost_cloud")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")
	data.Props["Footer"] = T("api.templates.copyright")

	if action == model.InviteLimitation {
		data.Props["Title"] = T("api.templates.upgrade_request_title", map[string]interface{}{"UserName": user})
		data.Props["Info4"] = T("api.templates.upgrade_request_info4")
	} else {
		data.Props["Title"] = T("api.templates.upgrade_request_title2")
		data.Props["Info4"] = T("api.templates.upgrade_request_info4_2")
	}

	body, err := es.srv.TemplatesContainer().RenderToString("cloud_upgrade_request_email", data)
	if err != nil {
		return false, model.NewAppError("SendUpgradeEmail", "api.user.send_upgrade_request_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendUpgradeEmail", "api.user.send_upgrade_request_email.error", nil, err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendOverUserLimitWarningEmail(email string, locale string, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_title")
	data.Props["Info1"] = T("api.templates.over_limit_info1")
	data.Props["Info2"] = T("api.templates.over_limit_info2")
	data.Props["Button"] = T("api.templates.upgrade_mattermost_cloud")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("reached_user_limit_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendOverUserLimitThirtyDayWarningEmail(email string, locale string, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_30_days_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_30_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_30_days_info1")
	data.Props["Info2"] = T("api.templates.over_limit_30_days_info2")
	data.Props["Info2Item1"] = T("api.templates.over_limit_30_days_info2_item1")
	data.Props["Info2Item2"] = T("api.templates.over_limit_30_days_info2_item2")
	data.Props["Info2Item3"] = T("api.templates.over_limit_30_days_info2_item3")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("over_user_limit_30_days_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendOverUserLimitNinetyDayWarningEmail(email string, locale string, siteURL string, overLimitDate string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_90_days_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_90_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_90_days_info1", map[string]interface{}{"OverLimitDate": overLimitDate})
	data.Props["Info2"] = T("api.templates.over_limit_90_days_info2")
	data.Props["Info3"] = T("api.templates.over_limit_90_days_info3")
	data.Props["Info4"] = T("api.templates.over_limit_90_days_info4")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("over_user_limit_90_days_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendOverUserLimitWorkspaceSuspendedWarningEmail(email string, locale string, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_suspended_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_suspended_title")
	data.Props["Info1"] = T("api.templates.over_limit_suspended_info1")
	data.Props["Info2"] = T("api.templates.over_limit_suspended_info2")
	data.Props["Button"] = T("api.templates.over_limit_suspended_contact_support")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("over_user_limit_workspace_suspended_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendOverUserFourteenDayWarningEmail(email string, locale string, siteURL string, overLimitDate string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_14_days_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_14_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_14_days_info1", map[string]interface{}{"OverLimitDate": overLimitDate})
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("over_user_limit_7_days_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendOverUserSevenDayWarningEmail(email string, locale string, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_7_days_subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_7_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_7_days_info1")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("over_user_limit_7_days_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendSuspensionEmailToSupport(email string, installationID string, customerID string, subscriptionID string, siteURL string, userCount int64) (bool, *model.AppError) {
	// Localization not needed

	subject := fmt.Sprintf("Cloud Installation %s Scheduled Suspension", installationID)
	data := es.newEmailTemplateData("en")
	data.Props["CustomerID"] = customerID
	data.Props["SiteURL"] = siteURL
	data.Props["SubscriptionID"] = subscriptionID
	data.Props["InstallationID"] = installationID
	data.Props["SuspensionDate"] = time.Now().AddDate(0, 0, 61).Format("2006-01-02")
	data.Props["UserCount"] = userCount

	body, err := es.srv.TemplatesContainer().RenderToString("over_user_limit_support_body", data)
	if err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendOverUserLimitWarningEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendPaymentFailedEmail(email string, locale string, failedPayment *model.FailedPayment, siteURL string) (bool, *model.AppError) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.payment_failed.subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.payment_failed.title")
	data.Props["Info1"] = T("api.templates.payment_failed.info1", map[string]interface{}{"CardBrand": failedPayment.CardBrand, "LastFour": failedPayment.LastFour})
	data.Props["Info2"] = T("api.templates.payment_failed.info2")
	data.Props["Info3"] = T("api.templates.payment_failed.info3")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	data.Props["FailedReason"] = failedPayment.FailureMessage

	body, err := es.srv.TemplatesContainer().RenderToString("payment_failed_body", data)
	if err != nil {
		return false, model.NewAppError("SendPaymentFailedEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, model.NewAppError("SendPaymentFailedEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return true, nil
}

func (es *EmailService) SendNoCardPaymentFailedEmail(email string, locale string, siteURL string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.payment_failed_no_card.subject")

	data := es.newEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.payment_failed_no_card.title")
	data.Props["Info1"] = T("api.templates.payment_failed_no_card.info1")
	data.Props["Info3"] = T("api.templates.payment_failed_no_card.info3")
	data.Props["Button"] = T("api.templates.payment_failed_no_card.button")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.srv.TemplatesContainer().RenderToString("payment_failed_no_card_body", data)
	if err != nil {
		return model.NewAppError("SendPaymentFailedEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return model.NewAppError("SendPaymentFailedEmail", "api.user.send_password_reset.send.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	return nil
}
