// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/shared/templates"
	"github.com/pkg/errors"
)

func (es *Service) SendChangeUsernameEmail(newUsername, email, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.username_change_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName,
			"TeamDisplayName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.username_change_body.title")
	data.Props["Info"] = T("api.templates.username_change_body.info",
		map[string]interface{}{"TeamDisplayName": es.config().TeamSettings.SiteName, "NewUsername": newUsername})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.templatesContainer.RenderToString("email_change_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendEmailChangeVerifyEmail(newUserEmail, locale, siteURL, token string) error {
	T := i18n.GetUserTranslations(locale)

	link := fmt.Sprintf("%s/do_verify_email?token=%s&email=%s", siteURL, token, url.QueryEscape(newUserEmail))

	subject := T("api.templates.email_change_verify_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName,
			"TeamDisplayName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.email_change_verify_body.title")
	data.Props["Info"] = T("api.templates.email_change_verify_body.info",
		map[string]interface{}{"TeamDisplayName": es.config().TeamSettings.SiteName})
	data.Props["VerifyUrl"] = link
	data.Props["VerifyButton"] = T("api.templates.email_change_verify_body.button")

	body, err := es.templatesContainer.RenderToString("email_change_verify_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(newUserEmail, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendEmailChangeEmail(oldEmail, newEmail, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.email_change_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName,
			"TeamDisplayName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.email_change_body.title")
	data.Props["Info"] = T("api.templates.email_change_body.info",
		map[string]interface{}{"TeamDisplayName": es.config().TeamSettings.SiteName, "NewEmail": newEmail})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.templatesContainer.RenderToString("email_change_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(oldEmail, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendVerifyEmail(userEmail, locale, siteURL, token, redirect string) error {
	T := i18n.GetUserTranslations(locale)

	link := fmt.Sprintf("%s/do_verify_email?token=%s&email=%s", siteURL, token, url.QueryEscape(userEmail))
	if redirect != "" {
		link += fmt.Sprintf("&redirect_to=%s", redirect)
	}

	serverURL := condenseSiteURL(siteURL)

	subject := T("api.templates.verify_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
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

	body, err := es.templatesContainer.RenderToString("verify_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendSignInChangeEmail(email, method, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.signin_change_email.subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.signin_change_email.body.title")
	data.Props["Info"] = T("api.templates.signin_change_email.body.info",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName, "Method": method})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.templatesContainer.RenderToString("signin_change_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendWelcomeEmail(userID string, email string, verified bool, disableWelcomeEmail bool, locale, siteURL, redirect string) error {
	if disableWelcomeEmail {
		return nil
	}
	if !*es.config().EmailSettings.SendEmailNotifications && !*es.config().EmailSettings.RequireEmailVerification {
		return errors.New("send email notifications and require email verification is disabled in the system console")
	}

	T := i18n.GetUserTranslations(locale)

	serverURL := condenseSiteURL(siteURL)

	subject := T("api.templates.welcome_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName,
			"ServerURL": serverURL})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.welcome_body.title")
	data.Props["SubTitle1"] = T("api.templates.welcome_body.subTitle1")
	data.Props["ServerURL"] = T("api.templates.welcome_body.serverURL", map[string]interface{}{"ServerURL": serverURL})
	data.Props["SubTitle2"] = T("api.templates.welcome_body.subTitle2")
	data.Props["Button"] = T("api.templates.welcome_body.button")
	data.Props["Info"] = T("api.templates.welcome_body.info")
	data.Props["Info1"] = T("api.templates.welcome_body.info1")
	data.Props["SiteURL"] = siteURL

	if *es.config().NativeAppSettings.AppDownloadLink != "" {
		data.Props["AppDownloadTitle"] = T("api.templates.welcome_body.app_download_title")
		data.Props["AppDownloadInfo"] = T("api.templates.welcome_body.app_download_info")
		data.Props["AppDownloadButton"] = T("api.templates.welcome_body.app_download_button")
		data.Props["AppDownloadLink"] = *es.config().NativeAppSettings.AppDownloadLink
	}

	if !verified && *es.config().EmailSettings.RequireEmailVerification {
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

	body, err := es.templatesContainer.RenderToString("welcome_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendCloudTrialEndWarningEmail(userEmail, name, trialEndDate, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.cloud_trial_ending_email.subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["Title"] = T("api.templates.cloud_trial_ending_email.title")
	data.Props["SubTitle"] = T("api.templates.cloud_trial_ending_email.subtitle", map[string]interface{}{"Name": name, "TrialEnd": trialEndDate})
	data.Props["SiteURL"] = siteURL
	data.Props["ButtonURL"] = fmt.Sprintf("%s/admin_console/billing/subscription?action=show_purchase_modal", siteURL)
	data.Props["Button"] = T("api.templates.cloud_trial_ending_email.add_payment_method")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.templatesContainer.RenderToString("cloud_trial_end_warning", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return err
	}
	return nil
}

func (es *Service) SendCloudTrialEndedEmail(userEmail, name, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.cloud_trial_ended_email.subject")

	t := time.Now()
	todayDate := fmt.Sprintf("%s %d, %d", t.Month(), t.Day(), t.Year())

	data := es.NewEmailTemplateData(locale)
	data.Props["Title"] = T("api.templates.cloud_trial_ended_email.title")
	data.Props["SubTitle"] = T("api.templates.cloud_trial_ended_email.subtitle", map[string]interface{}{"Name": name, "TodayDate": todayDate})
	data.Props["SiteURL"] = siteURL
	data.Props["ButtonURL"] = fmt.Sprintf("%s/admin_console/billing/subscription", siteURL)
	data.Props["Button"] = T("api.templates.cloud_trial_ended_email.start_subscription")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.templatesContainer.RenderToString("cloud_trial_ended_email", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return err
	}
	return nil
}

// SendCloudWelcomeEmail sends the cloud version of the welcome email
func (es *Service) SendCloudWelcomeEmail(userEmail, locale, teamInviteID, workSpaceName, dns, siteURL string) error {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.cloud_welcome_email.subject")

	data := es.NewEmailTemplateData(locale)
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

	body, err := es.templatesContainer.RenderToString("cloud_welcome_email", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(userEmail, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendPasswordChangeEmail(email, method, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.password_change_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName,
			"TeamDisplayName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.password_change_body.title")
	data.Props["Info"] = T("api.templates.password_change_body.info",
		map[string]interface{}{"TeamDisplayName": es.config().TeamSettings.SiteName, "TeamURL": siteURL, "Method": method})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.templatesContainer.RenderToString("password_change_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendUserAccessTokenAddedEmail(email, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.user_access_token_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.user_access_token_body.title")
	data.Props["Info"] = T("api.templates.user_access_token_body.info",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName, "SiteURL": siteURL})
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.templatesContainer.RenderToString("password_change_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendPasswordResetEmail(email string, token *model.Token, locale, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	link := fmt.Sprintf("%s/reset_password_complete?token=%s", siteURL, url.QueryEscape(token.Token))

	subject := T("api.templates.reset_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.reset_body.title")
	data.Props["SubTitle"] = T("api.templates.reset_body.subTitle")
	data.Props["Info"] = T("api.templates.reset_body.info")
	data.Props["ButtonURL"] = link
	data.Props["Button"] = T("api.templates.reset_body.button")
	data.Props["QuestionTitle"] = T("api.templates.questions_footer.title")
	data.Props["QuestionInfo"] = T("api.templates.questions_footer.info")

	body, err := es.templatesContainer.RenderToString("reset_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendMfaChangeEmail(email string, activated bool, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.mfa_change_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL

	if activated {
		data.Props["Info"] = T("api.templates.mfa_activated_body.info", map[string]interface{}{"SiteURL": siteURL})
		data.Props["Title"] = T("api.templates.mfa_activated_body.title")
	} else {
		data.Props["Info"] = T("api.templates.mfa_deactivated_body.info", map[string]interface{}{"SiteURL": siteURL})
		data.Props["Title"] = T("api.templates.mfa_deactivated_body.title")
	}
	data.Props["Warning"] = T("api.templates.email_warning")

	body, err := es.templatesContainer.RenderToString("mfa_change_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendInviteEmails(team *model.Team, senderName string, senderUserId string, invites []string, siteURL string) error {
	if es.PerHourEmailRateLimiter == nil {
		return NoRateLimiterError
	}
	rateLimited, result, err := es.PerHourEmailRateLimiter.RateLimit(senderUserId, len(invites))
	if err != nil {
		return SetupRateLimiterError
	}

	if rateLimited {
		mlog.Error("rate limit exceeded", mlog.Duration("RetryAfter", result.RetryAfter), mlog.Duration("ResetAfter", result.ResetAfter), mlog.String("user_id", senderUserId),
			mlog.String("team_id", team.Id), mlog.String("retry_after_secs", fmt.Sprintf("%f", result.RetryAfter.Seconds())), mlog.String("reset_after_secs", fmt.Sprintf("%f", result.ResetAfter.Seconds())))
		return RateLimitExceedError
	}

	for _, invite := range invites {
		if invite != "" {
			subject := i18n.T("api.templates.invite_subject",
				map[string]interface{}{"SenderName": senderName,
					"TeamDisplayName": team.DisplayName,
					"SiteName":        es.config().TeamSettings.SiteName})

			data := es.NewEmailTemplateData("")
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
				model.MapToJson(map[string]string{"teamId": team.Id, "email": invite}),
			)

			tokenProps := make(map[string]string)
			tokenProps["email"] = invite
			tokenProps["display_name"] = team.DisplayName
			tokenProps["name"] = team.Name
			tokenData := model.MapToJson(tokenProps)

			if err := es.store.Token().Save(token); err != nil {
				mlog.Error("Failed to send invite email successfully ", mlog.Err(err))
				continue
			}
			data.Props["ButtonURL"] = fmt.Sprintf("%s/signup_user_complete/?d=%s&t=%s", siteURL, url.QueryEscape(tokenData), url.QueryEscape(token.Token))

			body, err := es.templatesContainer.RenderToString("invite_body", data)
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

func (es *Service) SendGuestInviteEmails(team *model.Team, channels []*model.Channel, senderName string, senderUserId string, senderProfileImage []byte, invites []string, siteURL string, message string) error {
	if es.PerHourEmailRateLimiter == nil {
		return NoRateLimiterError
	}
	rateLimited, result, err := es.PerHourEmailRateLimiter.RateLimit(senderUserId, len(invites))
	if err != nil {
		return SetupRateLimiterError
	}

	if rateLimited {
		mlog.Error("rate limit exceeded", mlog.Duration("RetryAfter", result.RetryAfter), mlog.Duration("ResetAfter", result.ResetAfter), mlog.String("user_id", senderUserId),
			mlog.String("team_id", team.Id), mlog.String("retry_after_secs", fmt.Sprintf("%f", result.RetryAfter.Seconds())), mlog.String("reset_after_secs", fmt.Sprintf("%f", result.ResetAfter.Seconds())))
		return RateLimitExceedError
	}

	for _, invite := range invites {
		if invite != "" {
			subject := i18n.T("api.templates.invite_guest_subject",
				map[string]interface{}{"SenderName": senderName,
					"TeamDisplayName": team.DisplayName,
					"SiteName":        es.config().TeamSettings.SiteName})

			data := es.NewEmailTemplateData("")
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
				channelIDs = append(channelIDs, channel.Id)
			}

			token := model.NewToken(
				TokenTypeGuestInvitation,
				model.MapToJson(map[string]string{
					"teamId":   team.Id,
					"channels": strings.Join(channelIDs, " "),
					"email":    invite,
					"guest":    "true",
				}),
			)

			tokenProps := make(map[string]string)
			tokenProps["email"] = invite
			tokenProps["display_name"] = team.DisplayName
			tokenProps["name"] = team.Name
			tokenData := model.MapToJson(tokenProps)

			if err := es.store.Token().Save(token); err != nil {
				mlog.Error("Failed to send invite email successfully ", mlog.Err(err))
				continue
			}
			data.Props["ButtonURL"] = fmt.Sprintf("%s/signup_user_complete/?d=%s&t=%s", siteURL, url.QueryEscape(tokenData), url.QueryEscape(token.Token))

			if !*es.config().EmailSettings.SendEmailNotifications {
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

			body, err := es.templatesContainer.RenderToString("invite_body", data)
			if err != nil {
				mlog.Error("Failed to send invite email successfully", mlog.Err(err))
			}

			if nErr := es.SendMailWithEmbeddedFiles(invite, subject, body, embeddedFiles); nErr != nil {
				mlog.Error("Failed to send invite email successfully", mlog.Err(nErr))
			}
		}
	}
	return nil
}

func (es *Service) NewEmailTemplateData(locale string) templates.Data {
	var localT i18n.TranslateFunc
	if locale != "" {
		localT = i18n.GetUserTranslations(locale)
	} else {
		localT = i18n.T
	}
	organization := ""

	if *es.config().EmailSettings.FeedbackOrganization != "" {
		organization = localT("api.templates.email_organization") + *es.config().EmailSettings.FeedbackOrganization
	}

	return templates.Data{
		Props: map[string]interface{}{
			"EmailInfo1": localT("api.templates.email_info1"),
			"EmailInfo2": localT("api.templates.email_info2"),
			"EmailInfo3": localT("api.templates.email_info3",
				map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName}),
			"SupportEmail": *es.config().SupportSettings.SupportEmail,
			"Footer":       localT("api.templates.email_footer"),
			"FooterV2":     localT("api.templates.email_footer_v2"),
			"Organization": organization,
		},
		HTML: map[string]template.HTML{},
	}
}

func (es *Service) SendDeactivateAccountEmail(email string, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	serverURL := condenseSiteURL(siteURL)

	subject := T("api.templates.deactivate_subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName,
			"ServerURL": serverURL})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.deactivate_body.title", map[string]interface{}{"ServerURL": serverURL})
	data.Props["Info"] = T("api.templates.deactivate_body.info",
		map[string]interface{}{"SiteURL": siteURL})
	data.Props["Warning"] = T("api.templates.deactivate_body.warning")

	body, err := es.templatesContainer.RenderToString("deactivate_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

func (es *Service) SendNotificationMail(to, subject, htmlBody string) error {
	if !*es.config().EmailSettings.SendEmailNotifications {
		return nil
	}
	return es.sendMail(to, subject, htmlBody)
}

func (es *Service) sendMail(to, subject, htmlBody string) error {
	return es.sendMailWithCC(to, subject, htmlBody, "")
}

func (es *Service) sendMailWithCC(to, subject, htmlBody string, ccMail string) error {
	license := es.license()
	mailConfig := es.mailServiceConfig()

	return mail.SendMailUsingConfig(to, subject, htmlBody, mailConfig, license != nil && *license.Features.Compliance, ccMail)
}

func (es *Service) SendMailWithEmbeddedFiles(to, subject, htmlBody string, embeddedFiles map[string]io.Reader) error {
	license := es.license()
	mailConfig := es.mailServiceConfig()

	return mail.SendMailWithEmbeddedFilesUsingConfig(to, subject, htmlBody, embeddedFiles, mailConfig, license != nil && *license.Features.Compliance, "")
}

func (es *Service) CreateVerifyEmailToken(userID string, newEmail string) (*model.Token, error) {
	tokenExtra := struct {
		UserId string
		Email  string
	}{
		userID,
		newEmail,
	}

	jsonData, err := json.Marshal(tokenExtra)
	if err != nil {
		return nil, errors.Wrap(CreateEmailTokenError, err.Error())
	}

	token := model.NewToken(TokenTypeVerifyEmail, string(jsonData))

	if err = es.store.Token().Save(token); err != nil {
		return nil, err
	}

	return token, nil
}

func (es *Service) SendAtUserLimitWarningEmail(email string, locale string, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.at_limit_subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.at_limit_title")
	data.Props["Info1"] = T("api.templates.at_limit_info1")
	data.Props["Info2"] = T("api.templates.at_limit_info2")
	data.Props["Button"] = T("api.templates.upgrade_mattermost_cloud")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("reached_user_limit_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

// SendUpgradeEmail formats an email template and sends an email to an admin specified in the email arg
func (es *Service) SendUpgradeEmail(user, email, locale, siteURL, action string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.upgrade_request_subject")

	data := es.NewEmailTemplateData(locale)
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

	body, err := es.templatesContainer.RenderToString("cloud_upgrade_request_email", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendOverUserLimitWarningEmail(email string, locale string, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_title")
	data.Props["Info1"] = T("api.templates.over_limit_info1")
	data.Props["Info2"] = T("api.templates.over_limit_info2")
	data.Props["Button"] = T("api.templates.upgrade_mattermost_cloud")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("reached_user_limit_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendOverUserLimitThirtyDayWarningEmail(email string, locale string, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_30_days_subject")

	data := es.NewEmailTemplateData(locale)
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

	body, err := es.templatesContainer.RenderToString("over_user_limit_30_days_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendOverUserLimitNinetyDayWarningEmail(email string, locale string, siteURL string, overLimitDate string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_90_days_subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_90_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_90_days_info1", map[string]interface{}{"OverLimitDate": overLimitDate})
	data.Props["Info2"] = T("api.templates.over_limit_90_days_info2")
	data.Props["Info3"] = T("api.templates.over_limit_90_days_info3")
	data.Props["Info4"] = T("api.templates.over_limit_90_days_info4")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("over_user_limit_90_days_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendOverUserLimitWorkspaceSuspendedWarningEmail(email string, locale string, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_suspended_subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_suspended_title")
	data.Props["Info1"] = T("api.templates.over_limit_suspended_info1")
	data.Props["Info2"] = T("api.templates.over_limit_suspended_info2")
	data.Props["Button"] = T("api.templates.over_limit_suspended_contact_support")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("over_user_limit_workspace_suspended_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendOverUserFourteenDayWarningEmail(email string, locale string, siteURL string, overLimitDate string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_14_days_subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_14_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_14_days_info1", map[string]interface{}{"OverLimitDate": overLimitDate})
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("over_user_limit_7_days_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendOverUserSevenDayWarningEmail(email string, locale string, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.over_limit_7_days_subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.over_limit_7_days_title")
	data.Props["Info1"] = T("api.templates.over_limit_7_days_info1")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("over_user_limit_7_days_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendSuspensionEmailToSupport(email string, installationID string, customerID string, subscriptionID string, siteURL string, userCount int64) (bool, error) {
	// Localization not needed

	subject := fmt.Sprintf("Cloud Installation %s Scheduled Suspension", installationID)
	data := es.NewEmailTemplateData("en")
	data.Props["CustomerID"] = customerID
	data.Props["SiteURL"] = siteURL
	data.Props["SubscriptionID"] = subscriptionID
	data.Props["InstallationID"] = installationID
	data.Props["SuspensionDate"] = time.Now().AddDate(0, 0, 61).Format("2006-01-02")
	data.Props["UserCount"] = userCount

	body, err := es.templatesContainer.RenderToString("over_user_limit_support_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendPaymentFailedEmail(email string, locale string, failedPayment *model.FailedPayment, siteURL string) (bool, error) {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.payment_failed.subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.payment_failed.title")
	data.Props["Info1"] = T("api.templates.payment_failed.info1", map[string]interface{}{"CardBrand": failedPayment.CardBrand, "LastFour": failedPayment.LastFour})
	data.Props["Info2"] = T("api.templates.payment_failed.info2")
	data.Props["Info3"] = T("api.templates.payment_failed.info3")
	data.Props["Button"] = T("api.templates.over_limit_fix_now")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	data.Props["FailedReason"] = failedPayment.FailureMessage

	body, err := es.templatesContainer.RenderToString("payment_failed_body", data)
	if err != nil {
		return false, err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return false, err
	}

	return true, nil
}

func (es *Service) SendNoCardPaymentFailedEmail(email string, locale string, siteURL string) error {
	T := i18n.GetUserTranslations(locale)

	subject := T("api.templates.payment_failed_no_card.subject")

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.payment_failed_no_card.title")
	data.Props["Info1"] = T("api.templates.payment_failed_no_card.info1")
	data.Props["Info3"] = T("api.templates.payment_failed_no_card.info3")
	data.Props["Button"] = T("api.templates.payment_failed_no_card.button")
	data.Props["EmailUs"] = T("api.templates.email_us_anytime_at")

	data.Props["Footer"] = T("api.templates.copyright")

	body, err := es.templatesContainer.RenderToString("payment_failed_no_card_body", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}

// SendRemoveExpiredLicenseEmail formats an email and uses the email service to send the email to user with link pointing to CWS
// to renew the user license
func (es *Service) SendRemoveExpiredLicenseEmail(renewalLink, email string, locale, siteURL string) error {
	T := i18n.GetUserTranslations(locale)
	subject := T("api.templates.remove_expired_license.subject",
		map[string]interface{}{"SiteName": es.config().TeamSettings.SiteName})

	data := es.NewEmailTemplateData(locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = T("api.templates.remove_expired_license.body.title")
	data.Props["Link"] = renewalLink
	data.Props["LinkButton"] = T("api.templates.remove_expired_license.body.renew_button")

	body, err := es.templatesContainer.RenderToString("remove_expired_license", data)
	if err != nil {
		return err
	}

	if err := es.sendMail(email, subject, body); err != nil {
		return err
	}

	return nil
}
