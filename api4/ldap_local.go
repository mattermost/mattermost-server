// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitLdapLocal() {
	api.BaseRoutes.LDAP.Handle("/migrateid", api.ApiLocal(migrateIDLdap)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/sync", api.ApiLocal(syncLdap)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/test", api.ApiLocal(testLdap)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/groups", api.ApiLocal(getLdapGroups)).Methods("GET")
	api.BaseRoutes.LDAP.Handle("/certificate/public", api.ApiLocal(addLdapPublicCertificate)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/certificate/private", api.ApiLocal(addLdapPrivateCertificate)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/certificate/public", api.ApiLocal(removeLdapPublicCertificate)).Methods("DELETE")
	api.BaseRoutes.LDAP.Handle("/certificate/private", api.ApiLocal(removeLdapPrivateCertificate)).Methods("DELETE")

}
