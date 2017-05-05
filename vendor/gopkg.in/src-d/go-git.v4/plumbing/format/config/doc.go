// Package config implements decoding/encoding of git config files.
package config

/*

CONFIGURATION FILE
------------------

The Git configuration file contains a number of variables that affect
the Git commands' behavior. The `.git/config` file in each repository
is used to store the configuration for that repository, and
`$HOME/.gitconfig` is used to store a per-user configuration as
fallback values for the `.git/config` file. The file `/etc/gitconfig`
can be used to store a system-wide default configuration.

The configuration variables are used by both the Git plumbing
and the porcelains. The variables are divided into sections, wherein
the fully qualified variable name of the variable itself is the last
dot-separated segment and the section name is everything before the last
dot. The variable names are case-insensitive, allow only alphanumeric
characters and `-`, and must start with an alphabetic character.  Some
variables may appear multiple times; we say then that the variable is
multivalued.

Syntax
~~~~~~

The syntax is fairly flexible and permissive; whitespaces are mostly
ignored.  The '#' and ';' characters begin comments to the end of line,
blank lines are ignored.

The file consists of sections and variables.  A section begins with
the name of the section in square brackets and continues until the next
section begins.  Section names are case-insensitive.  Only alphanumeric
characters, `-` and `.` are allowed in section names.  Each variable
must belong to some section, which means that there must be a section
header before the first setting of a variable.

Sections can be further divided into subsections.  To begin a subsection
put its name in double quotes, separated by space from the section name,
in the section header, like in the example below:

--------
	[section "subsection"]

--------

Subsection names are case sensitive and can contain any characters except
newline (doublequote `"` and backslash can be included by escaping them
as `\"` and `\\`, respectively).  Section headers cannot span multiple
lines.  Variables may belong directly to a section or to a given subsection.
You can have `[section]` if you have `[section "subsection"]`, but you
don't need to.

There is also a deprecated `[section.subsection]` syntax. With this
syntax, the subsection name is converted to lower-case and is also
compared case sensitively. These subsection names follow the same
restrictions as section names.

All the other lines (and the remainder of the line after the section
header) are recognized as setting variables, in the form
'name = value' (or just 'name', which is a short-hand to say that
the variable is the boolean "true").
The variable names are case-insensitive, allow only alphanumeric characters
and `-`, and must start with an alphabetic character.

A line that defines a value can be continued to the next line by
ending it with a `\`; the backquote and the end-of-line are
stripped.  Leading whitespaces after 'name =', the remainder of the
line after the first comment character '#' or ';', and trailing
whitespaces of the line are discarded unless they are enclosed in
double quotes.  Internal whitespaces within the value are retained
verbatim.

Inside double quotes, double quote `"` and backslash `\` characters
must be escaped: use `\"` for `"` and `\\` for `\`.

The following escape sequences (beside `\"` and `\\`) are recognized:
`\n` for newline character (NL), `\t` for horizontal tabulation (HT, TAB)
and `\b` for backspace (BS).  Other char escape sequences (including octal
escape sequences) are invalid.


Includes
~~~~~~~~

You can include one config file from another by setting the special
`include.path` variable to the name of the file to be included. The
variable takes a pathname as its value, and is subject to tilde
expansion.

The
included file is expanded immediately, as if its contents had been
found at the location of the include directive. If the value of the
`include.path` variable is a relative path, the path is considered to be
relative to the configuration file in which the include directive was
found.  See below for examples.


Example
~~~~~~~

	# Core variables
	[core]
		; Don't trust file modes
		filemode = false

	# Our diff algorithm
	[diff]
		external = /usr/local/bin/diff-wrapper
		renames = true

	[branch "devel"]
		remote = origin
		merge = refs/heads/devel

	# Proxy settings
	[core]
		gitProxy="ssh" for "kernel.org"
		gitProxy=default-proxy ; for the rest

	[include]
		path = /path/to/foo.inc ; include by absolute path
		path = foo ; expand "foo" relative to the current file
		path = ~/foo ; expand "foo" in your `$HOME` directory


Values
~~~~~~

Values of many variables are treated as a simple string, but there
are variables that take values of specific types and there are rules
as to how to spell them.

boolean::

       When a variable is said to take a boolean value, many
       synonyms are accepted for 'true' and 'false'; these are all
       case-insensitive.

       true;; Boolean true can be spelled as `yes`, `on`, `true`,
		or `1`.  Also, a variable defined without `= <value>`
		is taken as true.

       false;; Boolean false can be spelled as `no`, `off`,
		`false`, or `0`.
+
When converting value to the canonical form using `--bool` type
specifier; 'git config' will ensure that the output is "true" or
"false" (spelled in lowercase).

integer::
       The value for many variables that specify various sizes can
       be suffixed with `k`, `M`,... to mean "scale the number by
       1024", "by 1024x1024", etc.

color::
       The value for a variable that takes a color is a list of
       colors (at most two, one for foreground and one for background)
       and attributes (as many as you want), separated by spaces.
+
The basic colors accepted are `normal`, `black`, `red`, `green`, `yellow`,
`blue`, `magenta`, `cyan` and `white`.  The first color given is the
foreground; the second is the background.
+
Colors may also be given as numbers between 0 and 255; these use ANSI
256-color mode (but note that not all terminals may support this).  If
your terminal supports it, you may also specify 24-bit RGB values as
hex, like `#ff0ab3`.
+

From: https://git-scm.com/docs/git-config
The accepted attributes are `bold`, `dim`, `ul`, `blink`, `reverse`,
`italic`, and `strike` (for crossed-out or "strikethrough" letters).
The position of any attributes with respect to the colors
(before, after, or in between), doesn't matter. Specific attributes may
be turned off by prefixing them with `no` or `no-` (e.g., `noreverse`,
`no-ul`, etc).
+
For git's pre-defined color slots, the attributes are meant to be reset
at the beginning of each item in the colored output. So setting
`color.decorate.branch` to `black` will paint that branch name in a
plain `black`, even if the previous thing on the same output line (e.g.
opening parenthesis before the list of branch names in `log --decorate`
output) is set to be painted with `bold` or some other attribute.
However, custom log formats may do more complicated and layered
coloring, and the negated forms may be useful there.

pathname::
	A variable that takes a pathname value can be given a
	string that begins with "`~/`" or "`~user/`", and the usual
	tilde expansion happens to such a string: `~/`
	is expanded to the value of `$HOME`, and `~user/` to the
	specified user's home directory.

From:
https://raw.githubusercontent.com/git/git/659889482ac63411daea38b2c3d127842ea04e4d/Documentation/config.txt

*/
