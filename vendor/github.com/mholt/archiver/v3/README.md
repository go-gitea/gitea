# archiver [![archiver GoDoc](https://img.shields.io/badge/reference-godoc-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/mholt/archiver?tab=doc) <a href="https://dev.azure.com/mholt-dev/Archiver/_build"><img src="https://img.shields.io/azure-devops/build/mholt-dev/1e14e7f7-f929-4fec-a1db-fa5a3c0d4ca9/2/master.svg?label=cross-platform%20tests&style=flat-square"></a>

Introducing **Archiver 3.1** - a cross-platform, multi-format archive utility and Go library. A powerful and flexible library meets an elegant CLI in this generic replacement for several platform-specific or format-specific archive utilities.

## Features

Package archiver makes it trivially easy to make and extract common archive formats such as tarball (and its compressed variants) and zip. Simply name the input and output file(s). The `arc` command runs the same on all platforms and has no external dependencies (not even libc). It is powered by the Go standard library and several third-party, pure-Go libraries.

Files are put into the root of the archive; directories are recursively added, preserving structure.

- Make whole archives from a list of files
- Open whole archives to a folder
- Extract specific files/folders from archives
- Stream files in and out of archives without needing actual files on disk
- Traverse archive contents without loading them
- Compress files
- Decompress files
- Streaming compression and decompression
- Several archive and compression formats supported

### Format-dependent features

- Gzip is multithreaded
- Optionally create a top-level folder to avoid littering a directory or archive root with files
- Toggle overwrite existing files
- Adjust compression level
- Zip: store (not compress) already-compressed files
- Make all necessary directories
- Open password-protected RAR archives
- Optionally continue with other files after an error

### Supported compression formats

- brotli (br)
- bzip2 (bz2)
- flate (zip)
- gzip (gz)
- lz4
- snappy (sz)
- xz
- zstandard (zstd)

### Supported archive formats

- .zip
- .tar (including any compressed variants like .tar.gz)
- .rar (read-only)

Tar files can optionally be compressed using any of the above compression formats.

## GoDoc

See <https://pkg.go.dev/github.com/mholt/archiver/v3>

## Install

### With webi

[`webi`](https://webinstall.dev/arc) will install `webi` and `arc` to `~/.local/bin/` and update your `PATH`.

#### Mac, Linux, Raspberry Pi

```bash
curl -fsS https://webinstall.dev/arc | bash
```

#### Windows 10

```pwsh
curl.exe -fsS -A MS https://webinstall.dev/arc | powershell
```

### With Go

To install the runnable binary to your \$GOPATH/bin:

```bash
go get github.com/mholt/archiver/cmd/arc
```

### Manually

To install manually

1. Download the binary for your platform from the [Github Releases](https://github.com/mholt/archiver/releases) page.
2. Move the binary to a location in your path, for example:
   - without `sudo`:
     ```bash
     chmod a+x ~/Downloads/arc_*
     mkdir -p ~/.local/bin
     mv ~/Downloads/arc_* ~/.local/bin/arc
     ```
   - as `root`:
     ```bash
     chmod a+x ~/Downloads/arc_*
     sudo mkdir -p /usr/local/bin
     sudo mv ~/Downloads/arc_* /usr/local/bin/arc
     ```
3. If needed, update `~/.bashrc` or `~/.profile` to include add `arc` in your `PATH`, for example:
   ```
   echo 'PATH="$HOME:/.local/bin:$PATH"' >> ~/.bashrc
   ```

## Build from Source

You can successfully build `arc` with just the go tooling, or with `goreleaser`.

### With `go`

```bash
go build cmd/arc/*.go
```

### Multi-platform with `goreleaser`

Builds with `goreleaser` will also include version info.

```bash
goreleaser --snapshot --skip-publish --rm-dist
```

## Command Use

### Make new archive

```bash
# Syntax: arc archive [archive name] [input files...]

arc archive test.tar.gz file1.txt images/file2.jpg folder/subfolder
```

(At least one input file is required.)

### Extract entire archive

```bash
# Syntax: arc unarchive [archive name] [destination]

arc unarchive test.tar.gz
```

(The destination path is optional; default is current directory.)

The archive name must end with a supported file extension&mdash;this is how it knows what kind of archive to make. Run `arc help` for more help.

### List archive contents

```bash
# Syntax: arc ls [archive name]

arc ls caddy_dist.tar.gz
```

```txt
drwxr-xr-x  matt    staff   0       2018-09-19 15:47:18 -0600 MDT   dist/
-rw-r--r--  matt    staff   6148    2017-08-07 18:34:22 -0600 MDT   dist/.DS_Store
-rw-r--r--  matt    staff   22481   2018-09-19 15:47:18 -0600 MDT   dist/CHANGES.txt
-rw-r--r--  matt    staff   17189   2018-09-19 15:47:18 -0600 MDT   dist/EULA.txt
-rw-r--r--  matt    staff   25261   2016-03-07 16:32:00 -0700 MST   dist/LICENSES.txt
-rw-r--r--  matt    staff   1017    2018-09-19 15:47:18 -0600 MDT   dist/README.txt
-rw-r--r--  matt    staff   288     2016-03-21 11:52:38 -0600 MDT   dist/gitcookie.sh.enc
...
```

### Extract a specific file or folder from an archive

```bash
# Syntax: arc extract [archive name] [path in archive] [destination on disk]

arc extract test.tar.gz foo/hello.txt extracted/hello.txt
```

### Compress a single file

```bash
# Syntax: arc compress [input file] [output file]

arc compress test.txt compressed_test.txt.gz
arc compress test.txt gz
```

For convenience, the output file (second argument) may simply be a compression format (without leading dot), in which case the output filename will be the same as the input filename but with the format extension appended, and the input file will be deleted if successful.

### Decompress a single file

```bash
# Syntax: arc decompress [input file] [output file]

arc decompress test.txt.gz original_test.txt
arc decompress test.txt.gz
```

For convenience, the output file (second argument) may be omitted. In that case, the output filename will have the same name as the input filename, but with the compression extension stripped from the end; and the input file will be deleted if successful.

### Flags

Flags are specified before the subcommand. Use `arc help` or `arc -h` to get usage help and a description of flags with their default values.

## Library Use

The archiver package allows you to easily create and open archives, walk their contents, extract specific files, compress and decompress files, and even stream archives in and out using pure io.Reader and io.Writer interfaces, without ever needing to touch the disk.

To use as a dependency in your project:

```bash
go get github.com/mholt/archiver/v3
```

```go
import "github.com/mholt/archiver/v3"
```

[See the package's GoDoc](https://pkg.go.dev/github.com/mholt/archiver?tab=doc) for full API documentation.

For example, creating or unpacking an archive file:

```go
err := archiver.Archive([]string{"testdata", "other/file.txt"}, "test.zip")
// ...
err = archiver.Unarchive("test.tar.gz", "test")
```

The archive format is determined by file extension. (There are [several functions in this package](https://pkg.go.dev/github.com/mholt/archiver?tab=doc) which perform a task by inferring the format from file extension or file header, including `Archive()`, `Unarchive()`, `CompressFile()`, and `DecompressFile()`.)

To configure the archiver used or perform, create an instance of the format's type:

```go
z := archiver.Zip{
	CompressionLevel:       flate.DefaultCompression,
	MkdirAll:               true,
	SelectiveCompression:   true,
	ContinueOnError:        false,
	OverwriteExisting:      false,
	ImplicitTopLevelFolder: false,
}

err := z.Archive([]string{"testdata", "other/file.txt"}, "/Users/matt/Desktop/test.zip")
```

Inspecting an archive:

```go
err = z.Walk("/Users/matt/Desktop/test.zip", func(f archiver.File) error {
	zfh, ok := f.Header.(zip.FileHeader)
	if ok {
		fmt.Println("Filename:", zfh.Name)
	}
	return nil
})
```

Streaming files into an archive that is being written to the HTTP response:

```go
err = z.Create(responseWriter)
if err != nil {
	return err
}
defer z.Close()

for _, fname := range filenames {
	info, err := os.Stat(fname)
	if err != nil {
		return err
	}

	// get file's name for the inside of the archive
	internalName, err := archiver.NameInArchive(info, fname, fname)
	if err != nil {
		return err
	}

	// open the file
	file, err := os.Open(f)
	if err != nil {
		return err
	}

	// write it to the archive
	err = z.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   info,
			CustomName: internalName,
		},
		ReadCloser: file,
	})
	file.Close()
	if err != nil {
		return err
	}
}
```

The `archiver.File` type allows you to use actual files with archives, or to mimic files when you only have streams.

There's a lot more that can be done, too. [See the GoDoc](https://pkg.go.dev/github.com/mholt/archiver?tab=doc) for full API documentation.

**Security note: This package does NOT attempt to mitigate zip-slip attacks.** It is [extremely difficult](https://github.com/rubyzip/rubyzip/pull/376) [to do properly](https://github.com/mholt/archiver/pull/65#issuecomment-395988244) and [seemingly impossible to mitigate effectively across platforms](https://github.com/golang/go/issues/20126). [Attempted fixes have broken processing of legitimate files in production](https://github.com/mholt/archiver/pull/70#issuecomment-423267320), rendering the program unusable. Our recommendation instead is to inspect the contents of an untrusted archive before extracting it (this package provides `Walkers`) and decide if you want to proceed with extraction.

## Project Values

This project has a few principle-based goals that guide its development:

- **Do our thing really well.** Our thing is creating, opening, inspecting, compressing, and streaming archive files. It is not meant to be a replacement for specific archive format tools like tar, zip, etc. that have lots of features and customizability. (Some customizability is OK, but not to the extent that it becomes overly complicated or error-prone.)

- **Have good tests.** Changes should be covered by tests.

- **Limit dependencies.** Keep the package lightweight.

- **Pure Go.** This means no cgo or other external/system dependencies. This package should be able to stand on its own and cross-compile easily to any platform -- and that includes its library dependencies.

- **Idiomatic Go.** Keep interfaces small, variable names semantic, vet shows no errors, the linter is generally quiet, etc.

- **Be elegant.** This package should be elegant to use and its code should be elegant when reading and testing. If it doesn't feel good, fix it up.

- **Well-documented.** Use comments prudently; explain why non-obvious code is necessary (and use tests to enforce it). Keep the docs updated, and have examples where helpful.

- **Keep it efficient.** This often means keep it simple. Fast code is valuable.

- **Consensus.** Contributions should ideally be approved by multiple reviewers before being merged. Generally, avoid merging multi-chunk changes that do not go through at least one or two iterations/reviews. Except for trivial changes, PRs are seldom ready to merge right away.

- **Have fun contributing.** Coding is awesome!

We welcome contributions and appreciate your efforts! However, please open issues to discuss any changes before spending the time preparing a pull request. This will save time, reduce frustration, and help coordinate the work. Thank you!
