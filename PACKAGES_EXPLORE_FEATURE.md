# Packages Explore Feature

## Overview
This feature adds a new "Packages" tab to the Explore page, allowing users to discover and browse packages that they have access to across the Gitea instance.

## Features

### User-Facing Features
1. **Packages Tab in Explore**: A new tab in the explore navigation that displays all accessible packages
2. **Search and Filter**: Users can search packages by name and filter by package type (npm, Maven, Docker, etc.)
3. **Permission-Based Access**: Only shows packages that the user has permission to view based on:
   - Public user packages (visible to everyone)
   - Limited visibility user packages (visible to logged-in users)
   - Organization packages (visible based on org visibility and membership)
   - Private packages (only visible to the owner)

### Admin Features
1. **Toggle Control**: Admins can enable/disable the packages explore page via `app.ini` configuration
2. **Configuration Setting**: `[service.explore]` section with `DISABLE_PACKAGES_PAGE` option

## Configuration

Add the following to your `app.ini` file under the `[service.explore]` section:

```ini
[service.explore]
; Disable the packages explore page
DISABLE_PACKAGES_PAGE = false
```

Set to `true` to hide the packages tab from the explore page.

## Implementation Details

### Backend Changes
- **New Handler**: `routers/web/explore/packages.go` - Handles package listing with permission filtering
- **Configuration**: `modules/setting/service.go` - Added `DisablePackagesPage` setting
- **Route**: Added `/explore/packages` route in `routers/web/web.go`

### Frontend Changes
- **Template**: `templates/explore/packages.tmpl` - Displays package list with search/filter
- **Navigation**: Updated `templates/explore/navbar.tmpl` to include packages tab

### Permission Logic
The feature implements proper access control by:
1. Fetching packages from the database
2. Checking each package's owner visibility:
   - For user-owned packages: Check user visibility (public/limited/private)
   - For org-owned packages: Check org visibility and user membership
3. Filtering results to only show accessible packages
4. Respecting the `DISABLE_PACKAGES_PAGE` configuration setting

## Security Considerations
- Anonymous users only see packages from public users/organizations
- Logged-in users see packages from public and limited visibility users, plus organizations they're members of
- Private user packages are only visible to the owner
- The feature requires packages to be enabled (`[packages] ENABLED = true`)

## Testing
To test the feature:
1. Enable packages in your Gitea instance
2. Create packages under different users/organizations with varying visibility settings
3. Access `/explore/packages` as different user types (anonymous, logged-in, org member)
4. Verify that only appropriate packages are displayed
5. Test the admin toggle by setting `DISABLE_PACKAGES_PAGE = true` and verifying the tab disappears

## Future Enhancements
Potential improvements for future versions:
- Add sorting options (by date, name, downloads)
- Implement more efficient database-level permission filtering
- Add package statistics and trending packages
- Support for package categories/tags
