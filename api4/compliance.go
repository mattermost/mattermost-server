// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"strconv"

	"github.com/avct/uasurfer"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitCompliance() {
	api.BaseRoutes.Compliance.Handle("/reports", api.APISessionRequired(createComplianceReport)).Methods("POST")
	api.BaseRoutes.Compliance.Handle("/reports", api.APISessionRequired(getComplianceReports)).Methods("GET")
	api.BaseRoutes.Compliance.Handle("/reports/{report_id:[A-Za-z0-9]+}", api.APISessionRequired(getComplianceReport)).Methods("GET")
	api.BaseRoutes.Compliance.Handle("/reports/{report_id:[A-Za-z0-9]+}/download", api.APISessionRequiredTrustRequester(downloadComplianceReport)).Methods("GET")
}

func createComplianceReport(c *Context, w http.ResponseWriter, r *http.Request) {
	job := model.ComplianceFromJSON(r.Body)
	if job == nil {
		c.SetInvalidParam("compliance")
		return
	}

	auditRec := c.MakeAuditRecord("createComplianceReport", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreateComplianceExportJob) {
		c.SetPermissionError(model.PermissionCreateComplianceExportJob)
		return
	}

	job.UserID = c.AppContext.Session().UserID

	rjob, err := c.App.SaveComplianceReport(job)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("compliance_id", rjob.ID)
	auditRec.AddMeta("compliance_desc", rjob.Desc)
	c.LogAudit("")

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(rjob.ToJSON()))
}

func getComplianceReports(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadComplianceExportJob) {
		c.SetPermissionError(model.PermissionReadComplianceExportJob)
		return
	}

	auditRec := c.MakeAuditRecord("getComplianceReports", audit.Fail)
	defer c.LogAuditRec(auditRec)

	crs, err := c.App.GetComplianceReports(c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	w.Write([]byte(crs.ToJSON()))
}

func getComplianceReport(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireReportID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("getComplianceReport", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadComplianceExportJob) {
		c.SetPermissionError(model.PermissionReadComplianceExportJob)
		return
	}

	job, err := c.App.GetComplianceReport(c.Params.ReportID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("compliance_id", job.ID)
	auditRec.AddMeta("compliance_desc", job.Desc)

	w.Write([]byte(job.ToJSON()))
}

func downloadComplianceReport(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireReportID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("downloadComplianceReport", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("compliance_id", c.Params.ReportID)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionDownloadComplianceExportResult) {
		c.SetPermissionError(model.PermissionDownloadComplianceExportResult)
		return
	}

	job, err := c.App.GetComplianceReport(c.Params.ReportID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("compliance_id", job.ID)
	auditRec.AddMeta("compliance_desc", job.Desc)

	reportBytes, err := c.App.GetComplianceFile(job)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("length", len(reportBytes))

	c.LogAudit("downloaded " + job.Desc)

	w.Header().Set("Cache-Control", "max-age=2592000, private")
	w.Header().Set("Content-Length", strconv.Itoa(len(reportBytes)))
	w.Header().Del("Content-Type") // Content-Type will be set automatically by the http writer

	// attach extra headers to trigger a download on IE, Edge, and Safari
	ua := uasurfer.Parse(r.UserAgent())

	w.Header().Set("Content-Disposition", "attachment;filename=\""+job.JobName()+".zip\"")

	if ua.Browser.Name == uasurfer.BrowserIE || ua.Browser.Name == uasurfer.BrowserSafari {
		// trim off anything before the final / so we just get the file's name
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	auditRec.Success()

	w.Write(reportBytes)
}
