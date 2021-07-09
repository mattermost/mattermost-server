// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitLdapLocal() {
	api.BaseRoutes.LDAP.Handle("/migrateid", api.APILocal(migrateIDLdap)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/sync", api.APILocal(syncLdap)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/test", api.APILocal(testLdap)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/groups", api.APILocal(getLdapGroups)).Methods("GET")
	api.BaseRoutes.LDAP.Handle("/certificate/public", api.APILocal(addLdapPublicCertificate)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/certificate/private", api.APILocal(addLdapPrivateCertificate)).Methods("POST")
	api.BaseRoutes.LDAP.Handle("/certificate/public", api.APILocal(removeLdapPublicCertificate)).Methods("DELETE")
	api.BaseRoutes.LDAP.Handle("/certificate/private", api.APILocal(removeLdapPrivateCertificate)).Methods("DELETE")

}
