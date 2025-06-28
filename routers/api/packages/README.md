# Gitea Package Registry

This document gives a brief overview how the package registry is organized in code.

## Structure

The package registry code is divided into multiple modules to split the functionality and make code reuse possible.

| Module | Description |
| - | - |
| `models/packages` | Common methods and models used by all registry types |
| `models/packages/<type>` | Methods used by specific registry type. There should be no need to use type specific models. |
| `modules/packages` | Common methods and types used by multiple registry types |
| `modules/packages/<type>` | Registry type specific methods and types (e.g. metadata extraction of package files) |
| `routers/api/packages` | Route definitions for all registry types |
| `routers/api/packages/<type>` | Route implementation for a specific registry type |
| `services/packages` | Helper methods used by registry types to handle common tasks like package creation and deletion in `routers` |
| `services/packages/<type>` | Registry type specific methods used by `routers` and `services` |

## Models

Every package registry implementation uses the same underlying models:

| Model | Description |
| - | - |
| `Package` | The root of a package providing values fixed for every version (e.g. the package name) |
| `PackageVersion` | A version of a package containing metadata (e.g. the package description) |
| `PackageFile` | A file of a package describing its content (e.g. file name) |
| `PackageBlob` | The content of a file (may be shared by multiple files) |
| `PackageProperty` | Additional properties attached to `Package`, `PackageVersion` or `PackageFile` (e.g. used if metadata is needed for routing) |

The following diagram shows the relationship between the models:
```
Package <1---*> PackageVersion <1---*> PackageFile <*---1> PackageBlob
```

## Adding a new package registry type

Before adding a new package registry type have a look at the existing implementation to get an impression of how it could work.
Most registry types offer endpoints to retrieve the metadata, upload and download package files.
The upload endpoint is often the heavy part because it must validate the uploaded blob, extract metadata and create the models.
The methods to validate and extract the metadata should be added in the `modules/packages/<type>` package.
If the upload is valid the methods in `services/packages` allow to store the upload and create the corresponding models.
It depends if the registry type allows multiple files per package version which method should be called:
- `CreatePackageAndAddFile`: error if package version already exists
- `CreatePackageOrAddFileToExisting`: error if file already exists
- `AddFileToExistingPackage`: error if package version does not exist or file already exists

`services/packages` also contains helper methods to download a file or to remove a package version.
There are no helper methods for metadata endpoints because they are very type specific.
