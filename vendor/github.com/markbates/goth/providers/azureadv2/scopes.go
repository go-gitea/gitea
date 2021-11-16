package azureadv2

type (
	// ScopeType are the well known scopes which can be requested
	ScopeType string
)

// OpenID Permissions
//
// You can use these permissions to specify artifacts that you want returned in Azure AD authorization and token
// requests. They are supported differently by the Azure AD v1.0 and v2.0 endpoints.
//
// With the Azure AD (v1.0) endpoint, only the openid permission is used. You specify it in the scope parameter in an
// authorization request to return an ID token when you use the OpenID Connect protocol to sign in a user to your app.
// For more information, see Authorize access to web applications using OpenID Connect and Azure Active Directory. To
// successfully return an ID token, you must also make sure that the User.Read permission is configured when you
// register your app.
//
// With the Azure AD v2.0 endpoint, you specify the offline_access permission in the scope parameter to explicitly
// request a refresh token when using the OAuth 2.0 or OpenID Connect protocols. With OpenID Connect, you specify the
// openid permission to request an ID token. You can also specify the email permission, profile permission, or both to
// return additional claims in the ID token. You do not need to specify User.Read to return an ID token with the v2.0
// endpoint. For more information, see OpenID Connect scopes.
const (
	// OpenIDScope shows on the work account consent page as the "Sign you in" permission, and on the personal Microsoft
	// account consent page as the "View your profile and connect to apps and services using your Microsoft account"
	// permission. With this permission, an app can receive a unique identifier for the user in the form of the sub
	// claim. It also gives the app access to the UserInfo endpoint. The openid scope can be used at the v2.0 token
	// endpoint to acquire ID tokens, which can be used to secure HTTP calls between different components of an app.
	OpenIDScope ScopeType = "openid"

	// EmailScope can be used with the openid scope and any others. It gives the app access to the user's primary
	// email address in the form of the email claim. The email claim is included in a token only if an email address is
	// associated with the user account, which is not always the case. If it uses the email scope, your app should be
	// prepared to handle a case in which the email claim does not exist in the token.
	EmailScope ScopeType = "email"

	// ProfileScope can be used with the openid scope and any others. It gives the app access to a substantial
	// amount of information about the user. The information it can access includes, but is not limited to, the user's
	// given name, surname, preferred username, and object ID. For a complete list of the profile claims available in
	// the id_tokens parameter for a specific user, see the v2.0 tokens reference:
	// https://docs.microsoft.com/en-us/azure/active-directory/develop/v2-id-and-access-tokens.
	ProfileScope ScopeType = "profile"

	// OfflineAccessScope gives your app access to resources on behalf of the user for an extended time. On the work
	// account consent page, this scope appears as the "Access your data anytime" permission. On the personal Microsoft
	// account consent page, it appears as the "Access your info anytime" permission. When a user approves the
	// offline_access scope, your app can receive refresh tokens from the v2.0 token endpoint. Refresh tokens are
	// long-lived. Your app can get new access tokens as older ones expire.
	//
	// If your app does not request the offline_access scope, it won't receive refresh tokens. This means that when you
	// redeem an authorization code in the OAuth 2.0 authorization code flow, you'll receive only an access token from
	// the /token endpoint. The access token is valid for a short time. The access token usually expires in one hour.
	// At that point, your app needs to redirect the user back to the /authorize endpoint to get a new authorization
	// code. During this redirect, depending on the type of app, the user might need to enter their credentials again
	// or consent again to permissions.
	OfflineAccessScope ScopeType = "offline_access"
)

// Calendar Permissions
//
// Calendars.Read.Shared and Calendars.ReadWrite.Shared are only valid for work or school accounts. All other
// permissions are valid for both Microsoft accounts and work or school accounts.
//
// See also https://developer.microsoft.com/en-us/graph/docs/concepts/permissions_reference
const (
	// CalendarsReadScope allows the app to read events in user calendars.
	CalendarsReadScope ScopeType = "Calendars.Read"

	// CalendarsReadSharedScope allows the app to read events in all calendars that the user can access, including
	// delegate and shared calendars.
	CalendarsReadSharedScope ScopeType = "Calendars.Read.Shared"

	// CalendarsReadWriteScope allows the app to create, read, update, and delete events in user calendars.
	CalendarsReadWriteScope ScopeType = "Calendars.ReadWrite"

	// CalendarsReadWriteSharedScope allows the app to create, read, update and delete events in all calendars the user
	// has permissions to access. This includes delegate and shared calendars.
	CalendarsReadWriteSharedScope ScopeType = "Calendars.ReadWrite.Shared"
)

// Contacts Permissions
//
// Only the Contacts.Read and Contacts.ReadWrite delegated permissions are valid for Microsoft accounts.
//
// See also https://developer.microsoft.com/en-us/graph/docs/concepts/permissions_reference
const (
	// ContactsReadScope allows the app to read contacts that the user has permissions to access, including the user's
	// own and shared contacts.
	ContactsReadScope ScopeType = "Contacts.Read"

	// ContactsReadSharedScope allows the app to read contacts that the user has permissions to access, including the
	// user's own and shared contacts.
	ContactsReadSharedScope ScopeType = "Contacts.Read.Shared"

	// ContactsReadWriteScope allows the app to create, read, update, and delete user contacts.
	ContactsReadWriteScope ScopeType = "Contacts.ReadWrite"

	// ContactsReadWriteSharedScope allows the app to create, read, update and delete contacts that the user has
	// permissions to, including the user's own and shared contacts.
	ContactsReadWriteSharedScope ScopeType = "Contacts.ReadWrite.Shared"
)

// Device Permissions
//
// The Device.Read and Device.Command delegated permissions are valid only for personal Microsoft accounts.
//
// See also https://developer.microsoft.com/en-us/graph/docs/concepts/permissions_reference
const (
	// DeviceReadScope allows the app to read a user's list of devices on behalf of the signed-in user.
	DeviceReadScope ScopeType = "Device.Read"

	// DeviceCommandScope allows the app to launch another app or communicate with another app on a user's device on
	// behalf of the signed-in user.
	DeviceCommandScope ScopeType = "Device.Command"
)

// Directory Permissions
//
// Directory permissions are not supported on Microsoft accounts.
//
// Directory permissions provide the highest level of privilege for accessing directory resources such as User, Group,
// and Device in an organization.
//
// They also exclusively control access to other directory resources like: organizational contacts, schema extension
// APIs, Privileged Identity Management (PIM) APIs, as well as many of the resources and APIs listed under the Azure
// Active Directory node in the v1.0 and beta API reference documentation. These include administrative units, directory
// roles, directory settings, policy, and many more.
//
// The Directory.ReadWrite.All permission grants the following privileges:
//  - Full read of all directory resources (both declared properties and navigation properties)
//  - Create and update users
//  - Disable and enable users (but not company administrator)
//  - Set user alternative security id (but not administrators)
//  - Create and update groups
//  - Manage group memberships
//  - Update group owner
//  - Manage license assignments
//  - Define schema extensions on applications
//  - Note: No rights to reset user passwords
//  - Note: No rights to delete resources (including users or groups)
//  - Note: Specifically excludes create or update for resources not listed above. This includes: application,
//    oAauth2Permissiongrant, appRoleAssignment, device, servicePrincipal, organization, domains, and so on.
//
// See also https://developer.microsoft.com/en-us/graph/docs/concepts/permissions_reference
const (
	// DirectoryReadAllScope allows the app to read data in your organization's directory, such as users, groups and
	// apps.
	//
	// Note: Users may consent to applications that require this permission if the application is registered in their
	// own organization’s tenant.
	//
	// requires admin consent
	DirectoryReadAllScope ScopeType = "Directory.Read.All"

	// DirectoryReadWriteAllScope allows the app to read and write data in your organization's directory, such as users,
	// and groups. It does not allow the app to delete users or groups, or reset user passwords.
	//
	// requires admin consent
	DirectoryReadWriteAllScope ScopeType = "Directory.ReadWrite.All"

	// DirectoryAccessAsUserAllScope allows the app to have the same access to information in the directory as the
	// signed-in user.
	//
	// requires admin consent
	DirectoryAccessAsUserAllScope ScopeType = "Directory.AccessAsUser.All"
)

// Education Administration Permissions
const (
	// EduAdministrationReadScope allows the app to read education app settings on behalf of the user.
	//
	// requires admin consent
	EduAdministrationReadScope ScopeType = "EduAdministration.Read"

	// EduAdministrationReadWriteScope allows the app to manage education app settings on behalf of the user.
	//
	// requires admin consent
	EduAdministrationReadWriteScope ScopeType = "EduAdministration.ReadWrite"

	// EduAssignmentsReadBasicScope allows the app to read assignments without grades on behalf of the user
	//
	// requires admin consent
	EduAssignmentsReadBasicScope ScopeType = "EduAssignments.ReadBasic"

	// EduAssignmentsReadWriteBasicScope allows the app to read and write assignments without grades on behalf of the
	// user
	EduAssignmentsReadWriteBasicScope ScopeType = "EduAssignments.ReadWriteBasic"

	// EduAssignmentsReadScope allows the app to read assignments and their grades on behalf of the user
	//
	// requires admin consent
	EduAssignmentsReadScope ScopeType = "EduAssignments.Read"

	// EduAssignmentsReadWriteScope allows the app to read and write assignments and their grades on behalf of the user
	//
	// requires admin consent
	EduAssignmentsReadWriteScope ScopeType = "EduAssignments.ReadWrite"

	// EduRosteringReadBasicScope allows the app to read a limited subset of the data from the  structure of schools and
	// classes in an organization's roster and  education-specific information about users to be read on behalf of the
	// user.
	//
	// requires admin consent
	EduRosteringReadBasicScope ScopeType = "EduRostering.ReadBasic"
)

// Files Permissions
//
// The Files.Read, Files.ReadWrite, Files.Read.All, and Files.ReadWrite.All delegated permissions are valid on both
// personal Microsoft accounts and work or school accounts. Note that for personal accounts, Files.Read and
// Files.ReadWrite also grant access to files shared with the signed-in user.
//
// The Files.Read.Selected and Files.ReadWrite.Selected delegated permissions are only valid on work or school accounts
// and are only exposed for working with Office 365 file handlers (v1.0)
// https://msdn.microsoft.com/office/office365/howto/using-cross-suite-apps. They should not be used for directly
// calling Microsoft Graph APIs.
//
// The Files.ReadWrite.AppFolder delegated permission is only valid for personal accounts and is used for accessing the
// App Root special folder https://dev.onedrive.com/misc/appfolder.htm with the OneDrive Get special folder
// https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/drive_get_specialfolder Microsoft Graph API.
const (
	// FilesReadScope allows the app to read the signed-in user's files.
	FilesReadScope ScopeType = "Files.Read"

	// FilesReadAllScope allows the app to read all files the signed-in user can access.
	FilesReadAllScope ScopeType = "Files.Read.All"

	// FilesReadWrite allows the app to read, create, update, and delete the signed-in user's files.
	FilesReadWriteScope ScopeType = "Files.ReadWrite"

	// FilesReadWriteAllScope allows the app to read, create, update, and delete all files the signed-in user can access.
	FilesReadWriteAllScope ScopeType = "Files.ReadWrite.All"

	// FilesReadWriteAppFolderScope allows the app to read, create, update, and delete files in the application's folder.
	FilesReadWriteAppFolderScope ScopeType = "Files.ReadWrite.AppFolder"

	// FilesReadSelectedScope allows the app to read files that the user selects. The app has access for several hours
	// after the user selects a file.
	//
	// preview
	FilesReadSelectedScope ScopeType = "Files.Read.Selected"

	// FilesReadWriteSelectedScope allows the app to read and write files that the user selects. The app has access for
	// several hours after the user selects a file
	//
	// preview
	FilesReadWriteSelectedScope ScopeType = "Files.ReadWrite.Selected"
)

// Group Permissions
//
// Group functionality is not supported on personal Microsoft accounts.
//
// For Office 365 groups, Group permissions grant the app access to the contents of the group; for example,
// conversations, files, notes, and so on.
//
// For application permissions, there are some limitations for the APIs that are supported. For more information, see
// known issues.
//
// In some cases, an app may need Directory permissions to read some group properties like member and memberOf. For
// example, if a group has a one or more servicePrincipals as members, the app will need effective permissions to read
// service principals through being granted one of the Directory.* permissions, otherwise Microsoft Graph will return an
// error. (In the case of delegated permissions, the signed-in user will also need sufficient privileges in the
// organization to read service principals.) The same guidance applies for the memberOf property, which can return
// administrativeUnits.
//
// Group permissions are also used to control access to Microsoft Planner resources and APIs. Only delegated permissions
// are supported for Microsoft Planner APIs; application permissions are not supported. Personal Microsoft accounts are
// not supported.
const (
	// GroupReadAllScope allows the app to list groups, and to read their properties and all group memberships on behalf
	// of the signed-in user.  Also allows the app to read calendar, conversations, files, and other group content for
	// all groups the signed-in user can access.
	GroupReadAllScope ScopeType = "Group.Read.All"

	// GroupReadWriteAllScope allows the app to create groups and read all group properties and memberships on behalf of
	// the signed-in user.  Additionally allows group owners to manage their groups and allows group members to update
	// group content.
	GroupReadWriteAllScope ScopeType = "Group.ReadWrite.All"
)

// Identity Risk Event Permissions
//
// IdentityRiskEvent.Read.All is valid only for work or school accounts. For an app with delegated permissions to read
// identity risk information, the signed-in user must be a member of one of the following administrator roles: Global
// Administrator, Security Administrator, or Security Reader. For more information about administrator roles, see
// Assigning administrator roles in Azure Active Directory.
const (
	// IdentityRiskEventReadAllScope allows the app to read identity risk event information for all users in your
	// organization on behalf of the signed-in user.
	//
	// requires admin consent
	IdentityRiskEventReadAllScope ScopeType = "IdentityRiskEvent.Read.All"
)

// Identity Provider Permissions
//
// IdentityProvider.Read.All and IdentityProvider.ReadWrite.All are valid only for work or school accounts. For an app
// to read or write identity providers with delegated permissions, the signed-in user must be assigned the Global
// Administrator role. For more information about administrator roles, see Assigning administrator roles in Azure Active
// Directory.
const (
	// IdentityProviderReadAllScope allows the app to read identity providers configured in your Azure AD or Azure AD
	// B2C tenant on behalf of the signed-in user.
	//
	// requires admin consent
	IdentityProviderReadAllScope ScopeType = "IdentityProvider.Read.All"

	// IdentityProviderReadWriteAllScope allows the app to read or write identity providers configured in your Azure AD
	// or Azure AD B2C tenant on behalf of the signed-in user.
	//
	// requires admin consent
	IdentityProviderReadWriteAllScope ScopeType = "IdentityProvider.ReadWrite.All"
)

// Device Management Permissions
//
// Using the Microsoft Graph APIs to configure Intune controls and policies still requires that the Intune service is
// correctly licensed by the customer.
//
// These permissions are only valid for work or school accounts.
const (
	// DeviceManagementAppsReadAllScope allows the app to read the properties, group assignments and status of apps, app
	// configurations and app protection policies managed by Microsoft Intune.
	//
	// requires admin consent
	DeviceManagementAppsReadAllScope ScopeType = "DeviceManagementApps.Read.All"

	// DeviceManagementAppsReadWriteAllScope allows the app to read and write the properties, group assignments and
	// status of apps, app configurations and app protection policies managed by Microsoft Intune.
	//
	// requires admin consent
	DeviceManagementAppsReadWriteAllScope ScopeType = "DeviceManagementApps.ReadWrite.All"

	// DeviceManagementConfigurationReadAllScope allows the app to read properties of Microsoft Intune-managed device
	// configuration and device compliance policies and their assignment to groups.
	//
	// requires admin consent
	DeviceManagementConfigurationReadAllScope ScopeType = "DeviceManagementConfiguration.Read.All"

	// DeviceManagementConfigurationReadWriteAllScope allows the app to read and write properties of Microsoft
	// Intune-managed device configuration and device compliance policies and their assignment to groups.
	//
	// requires admin consent
	DeviceManagementConfigurationReadWriteAllScope ScopeType = "DeviceManagementConfiguration.ReadWrite.All"

	// DeviceManagementManagedDevicesPrivilegedOperationsAllScope allows the app to perform remote high impact actions
	// such as wiping the device or resetting the passcode on devices managed by Microsoft Intune.
	//
	// requires admin consent
	DeviceManagementManagedDevicesPrivilegedOperationsAllScope ScopeType = "DeviceManagementManagedDevices.PrivilegedOperations.All"

	// DeviceManagementManagedDevicesReadAllScope allows the app to read the properties of devices managed by Microsoft
	// Intune.
	//
	// requires admin consent
	DeviceManagementManagedDevicesReadAllScope ScopeType = "DeviceManagementManagedDevices.Read.All"

	// DeviceManagementManagedDevicesReadWriteAllScope allows the app to read and write the properties of devices
	// managed by Microsoft Intune. Does not allow high impact operations such as remote wipe and password reset on the
	// device’s owner.
	//
	// requires admin consent
	DeviceManagementManagedDevicesReadWriteAllScope ScopeType = "DeviceManagementManagedDevices.ReadWrite.All"

	// DeviceManagementRBACReadAllScope allows the app to read the properties relating to the Microsoft Intune
	// Role-Based Access Control (RBAC) settings.
	//
	// requires admin consent
	DeviceManagementRBACReadAllScope ScopeType = "DeviceManagementRBAC.Read.All"

	// DeviceManagementRBACReadWriteAllScope allows the app to read and write the properties relating to the Microsoft
	// Intune Role-Based Access Control (RBAC) settings.
	//
	// requires admin consent
	DeviceManagementRBACReadWriteAllScope ScopeType = "DeviceManagementRBAC.ReadWrite.All"

	// DeviceManagementServiceConfigReadAllScope allows the app to read Intune service properties including device
	// enrollment and third party service connection configuration.
	//
	// requires admin consent
	DeviceManagementServiceConfigReadAllScope ScopeType = "DeviceManagementServiceConfig.Read.All"

	// DeviceManagementServiceConfigReadWriteAllScope allows the app to read and write Microsoft Intune service
	// properties including device enrollment and third party service connection configuration.
	//
	// requires admin consent
	DeviceManagementServiceConfigReadWriteAllScope ScopeType = "DeviceManagementServiceConfig.ReadWrite.All"
)

// Mail Permissions
//
// Mail.Read.Shared, Mail.ReadWrite.Shared, and Mail.Send.Shared are only valid for work or school accounts. All other
// permissions are valid for both Microsoft accounts and work or school accounts.
//
// With the Mail.Send or Mail.Send.Shared permission, an app can send mail and save a copy to the user's Sent Items
// folder, even if the app does not use a corresponding Mail.ReadWrite or Mail.ReadWrite.Shared permission.
const (
	// MailReadScope allows the app to read email in user mailboxes.
	MailReadScope ScopeType = "Mail.Read"

	// MailReadWriteScope allows the app to create, read, update, and delete email in user mailboxes. Does not include
	// permission to send mail.
	MailReadWriteScope ScopeType = "Mail.ReadWrite"

	// MailReadSharedScope allows the app to read mail that the user can access, including the user's own and shared
	// mail.
	MailReadSharedScope ScopeType = "Mail.Read.Shared"

	// MailReadWriteSharedScope allows the app to create, read, update, and delete mail that the user has permission to
	// access, including the user's own and shared mail. Does not include permission to send mail.
	MailReadWriteSharedScope ScopeType = "Mail.ReadWrite.Shared"

	// MailSend allowsScope the app to send mail as users in the organization.
	MailSendScope ScopeType = "Mail.Send"

	// MailSendSharedScope allows the app to send mail as the signed-in user, including sending on-behalf of others.
	MailSendSharedScope ScopeType = "Mail.Send.Shared"

	// MailboxSettingsReadScope allows the app to the read user's mailbox settings. Does not include permission to send
	// mail.
	MailboxSettingsReadScope ScopeType = "Mailbox.Settings.Read"

	// MailboxSettingsReadWriteScope allows the app to create, read, update, and delete user's mailbox settings. Does
	// not include permission to directly send mail, but allows the app to create rules that can forward or redirect
	// messages.
	MailboxSettingsReadWriteScope ScopeType = "MailboxSettings.ReadWrite"
)

// Member Permissions
//
// Member.Read.Hidden is valid only on work or school accounts.
//
// Membership in some Office 365 groups can be hidden. This means that only the members of the group can view its
// members. This feature can be used to help comply with regulations that require an organization to hide group
// membership from outsiders (for example, an Office 365 group that represents students enrolled in a class).
const (
	// MemberReadHiddenScope allows the app to read the memberships of hidden groups and administrative units on behalf
	// of the signed-in user, for those hidden groups and administrative units that the signed-in user has access to.
	//
	// requires admin consent
	MemberReadHiddenScope ScopeType = "Member.Read.Hidden"
)

// Notes Permissions
//
// Notes.Read.All and Notes.ReadWrite.All are only valid for work or school accounts. All other permissions are valid
// for both Microsoft accounts and work or school accounts.
//
// With the Notes.Create permission, an app can view the OneNote notebook hierarchy of the signed-in user and create
// OneNote content (notebooks, section groups, sections, pages, etc.).
//
// Notes.ReadWrite and Notes.ReadWrite.All also allow the app to modify the permissions on the OneNote content that can
// be accessed by the signed-in user.
//
// For work or school accounts, Notes.Read.All and Notes.ReadWrite.All allow the app to access other users' OneNote
// content that the signed-in user has permission to within the organization.
const (
	// NotesReadScope allows the app to read OneNote notebooks on behalf of the signed-in user.
	NotesReadScope ScopeType = "Notes.Read"

	// NotesCreateScope allows the app to read the titles of OneNote notebooks and sections and to create new pages,
	// notebooks, and sections on behalf of the signed-in user.
	NotesCreateScope ScopeType = "Notes.Create"

	// NotesReadWriteScope allows the app to read, share, and modify OneNote notebooks on behalf of the signed-in user.
	NotesReadWriteScope ScopeType = "Notes.ReadWrite"

	// NotesReadAllScope allows the app to read OneNote notebooks that the signed-in user has access to in the
	// organization.
	NotesReadAllScope ScopeType = "Notes.Read.All"

	// NotesReadWriteAllScope allows the app to read, share, and modify OneNote notebooks that the signed-in user has
	// access to in the organization.
	NotesReadWriteAllScope ScopeType = "Notes.ReadWrite.All"
)

// People Permissions
//
// The People.Read.All permission is only valid for work and school accounts.
const (
	// PeopleReadScope allows the app to read a scored list of people relevant to the signed-in user. The list can
	// include local contacts, contacts from social networking or your organization's directory, and people from recent
	// communications (such as email and Skype).
	PeopleReadScope ScopeType = "People.Read"

	// PeopleReadAllScope allows the app to read a scored list of people relevant to the signed-in user or other users
	// in the signed-in user's organization. The list can include local contacts, contacts from social networking or
	// your organization's directory, and people from recent communications (such as email and Skype). Also allows the
	// app to search the entire directory of the signed-in user's organization.
	//
	// requires admin consent
	PeopleReadAllScope ScopeType = "People.Read.All"
)

// Report Permissions
//
// Reports permissions are only valid for work or school accounts.
const (
	// ReportsReadAllScope allows an app to read all service usage reports without a signed-in user. Services that
	// provide usage reports include Office 365 and Azure Active Directory.
	//
	// requires admin consent
	ReportsReadAllScope ScopeType = "Reports.Read.All"
)

// Security Permissions
//
// Security permissions are valid only on work or school accounts.
const (
	// SecurityEventsReadAllScope allows the app to read your organization’s security events on behalf of the signed-in
	// user.
	// requires admin consent
	SecurityEventsReadAllScope ScopeType = "SecurityEvents.Read.All"

	// SecurityEventsReadWriteAllScope allows the app to read your organization’s security events on behalf of the
	// signed-in user. Also allows the app to update editable properties in security events on behalf of the signed-in
	// user.
	//
	// requires admin consent
	SecurityEventsReadWriteAllScope ScopeType = "SecurityEvents.ReadWrite.All"
)

// Sites Permissions
//
// Sites permissions are valid only on work or school accounts.
const (
	// SitesReadAllScope allows the app to read documents and list items in all site collections on behalf of the
	// signed-in user.
	SitesReadAllScope ScopeType = "Sites.Read.All"

	// SitesReadWriteAllScope allows the app to edit or delete documents and list items in all site collections on
	// behalf of the signed-in user.
	SitesReadWriteAllScope ScopeType = "Sites.ReadWrite.All"

	// SitesManageAllScope allows the app to manage and create lists, documents, and list items in all site collections
	// on behalf of the signed-in user.
	SitesManageAllScope ScopeType = "Sites.Manage.All"

	// SitesFullControlAllScope allows the app to have full control to SharePoint sites in all site collections on
	// behalf of the signed-in user.
	//
	// requires admin consent
	SitesFullControlAllScope ScopeType = "Sites.FullControl.All"
)

// Tasks Permissions
//
// Tasks permissions are used to control access for Outlook tasks. Access for Microsoft Planner tasks is controlled by
// Group permissions.
//
// Shared permissions are currently only supported for work or school accounts. Even with Shared permissions, reads and
// writes may fail if the user who owns the shared content has not granted the accessing user permissions to modify
// content within the folder.
const (
	// TasksReadScope allows the app to read user tasks.
	TasksReadScope ScopeType = "Tasks.Read"

	// TasksReadSharedScope allows the app to read tasks a user has permissions to access, including their own and
	// shared tasks.
	TasksReadSharedScope ScopeType = "Tasks.Read.Shared"

	// TasksReadWriteScope allows the app to create, read, update and delete tasks and containers (and tasks in them)
	// that are assigned to or shared with the signed-in user.
	TasksReadWriteScope ScopeType = "Tasks.ReadWrite"

	// TasksReadWriteSharedScope allows the app to create, read, update, and delete tasks a user has permissions to,
	// including their own and shared tasks.
	TasksReadWriteSharedScope ScopeType = "Tasks.ReadWrite.Shared"
)

// Terms of Use Permissions
//
// All the permissions above are valid only for work or school accounts.
//
// For an app to read or write all agreements or agreement acceptances with delegated permissions, the signed-in user
// must be assigned the Global Administrator, Conditional Access Administrator or Security Administrator role. For more
// information about administrator roles, see Assigning administrator roles in Azure Active Directory
// https://docs.microsoft.com/azure/active-directory/active-directory-assign-admin-roles.
const (
	// AgreementReadAllScope allows the app to read terms of use agreements on behalf of the signed-in user.
	//
	// requires admin consent
	AgreementReadAllScope ScopeType = "Agreement.Read.All"

	// AgreementReadWriteAllScope allows the app to read and write terms of use agreements on behalf of the signed-in
	// user.
	//
	// requires admin consent
	AgreementReadWriteAllScope ScopeType = "Agreement.ReadWrite.All"

	// AgreementAcceptanceReadScope allows the app to read terms of use acceptance statuses on behalf of the signed-in
	// user.
	//
	// requires admin consent
	AgreementAcceptanceReadScope ScopeType = "AgreementAcceptance.Read"

	// AgreementAcceptanceReadAllScope allows the app to read terms of use acceptance statuses on behalf of the
	// signed-in user.
	//
	// requires admin consent
	AgreementAcceptanceReadAllScope ScopeType = "AgreementAcceptance.Read.All"
)

// User Permissions
//
// The only permissions valid for Microsoft accounts are User.Read and User.ReadWrite. For work or school accounts, all
// permissions are valid.
//
// With the User.Read permission, an app can also read the basic company information of the signed-in user for a work or
// school account through the organization resource. The following properties are available: id, displayName, and
// verifiedDomains.
//
// For work or school accounts, the full profile includes all of the declared properties of the User resource. On reads,
// only a limited number of properties are returned by default. To read properties that are not in the default set, use
// $select. The default properties are:
//  displayName
//  givenName
//  jobTitle
//  mail
//  mobilePhone
//  officeLocation
//  preferredLanguage
//  surname
//  userPrincipalName
//
// User.ReadWrite and User.Readwrite.All delegated permissions allow the app to update the following profile properties
// for work or school accounts:
//  aboutMe
//  birthday
//  hireDate
//  interests
//  mobilePhone
//  mySite
//  pastProjects
//  photo
//  preferredName
//  responsibilities
//  schools
//  skills
//
// With the User.ReadWrite.All application permission, the app can update all of the declared properties of work or
// school accounts except for password.
//
// To read or write direct reports (directReports) or the manager (manager) of a work or school account, the app must
// have either User.Read.All (read only) or User.ReadWrite.All.
//
// The User.ReadBasic.All permission constrains app access to a limited set of properties known as the basic profile.
// This is because the full profile might contain sensitive directory information. The basic profile includes only the
// following properties:
//  displayName
//  givenName
//  mail
//  photo
//  surname
//  userPrincipalName
//
// To read the group memberships of a user (memberOf), the app must have either Group.Read.All or Group.ReadWrite.All.
// However, if the user also has membership in a directoryRole or an administrativeUnit, the app will need effective
// permissions to read those resources too, or Microsoft Graph will return an error. This means the app will also need
// Directory permissions, and, for delegated permissions, the signed-in user will also need sufficient privileges in the
// organization to access directory roles and administrative units.
const (
	// UserReadScope allows users to sign-in to the app, and allows the app to read the profile of signed-in users. It
	// also allows the app to read basic company information of signed-in users.
	UserReadScope ScopeType = "User.Read"

	// UserReadWriteScope allows the app to read the signed-in user's full profile. It also allows the app to update the
	// signed-in user's profile information on their behalf.
	UserReadWriteScope ScopeType = "User.ReadWrite"

	// UserReadBasicAllScope allows the app to read a basic set of profile properties of other users in your
	// organization on behalf of the signed-in user. This includes display name, first and last name, email address,
	// open extensions and photo. Also allows the app to read the full profile of the signed-in user.
	UserReadBasicAllScope ScopeType = "User.ReadBasic.All"

	// UserReadAllScope allows the app to read the full set of profile properties, reports, and managers of other users
	// in your organization, on behalf of the signed-in user.
	//
	// requires admin consent
	UserReadAllScope ScopeType = "User.Read.All"

	// UserReadWriteAllScope allows the app to read and write the full set of profile properties, reports, and managers
	// of other users in your organization, on behalf of the signed-in user. Also allows the app to create and delete
	// users as well as reset user passwords on behalf of the signed-in user.
	//
	// requires admin consent
	UserReadWriteAllScope ScopeType = "User.ReadWrite.All"

	// UserInviteAllScope allows the app to invite guest users to your organization, on behalf of the signed-in user.
	//
	// requires admin consent
	UserInviteAllScope ScopeType = "User.Invite.All"

	// UserExportAllScope allows the app to export an organizational user's data, when performed by a Company
	// Administrator.
	//
	// requires admin consent
	UserExportAllScope ScopeType = "User.Export.All"
)

// User Activity Permissions
//
// UserActivity.ReadWrite.CreatedByApp is valid for both Microsoft accounts and work or school accounts.
//
// The CreatedByApp constraint associated with this permission indicates the service will apply implicit filtering to
// results based on the identity of the calling app, either the MSA app id or a set of app ids configured for a
// cross-platform application identity.
const (
	// UserActivityReadWriteCreatedByAppScope allows the app to read and report the signed-in user's activity in the
	// app.
	UserActivityReadWriteCreatedByAppScope ScopeType = "UserActivity.ReadWrite.CreatedByApp"
)
