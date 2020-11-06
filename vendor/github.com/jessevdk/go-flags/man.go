package flags

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"
)

func manQuote(s string) string {
	return strings.Replace(s, "\\", "\\\\", -1)
}

func formatForMan(wr io.Writer, s string) {
	for {
		idx := strings.IndexRune(s, '`')

		if idx < 0 {
			fmt.Fprintf(wr, "%s", manQuote(s))
			break
		}

		fmt.Fprintf(wr, "%s", manQuote(s[:idx]))

		s = s[idx+1:]
		idx = strings.IndexRune(s, '\'')

		if idx < 0 {
			fmt.Fprintf(wr, "%s", manQuote(s))
			break
		}

		fmt.Fprintf(wr, "\\fB%s\\fP", manQuote(s[:idx]))
		s = s[idx+1:]
	}
}

func writeManPageOptions(wr io.Writer, grp *Group) {
	grp.eachGroup(func(group *Group) {
		if group.Hidden || len(group.options) == 0 {
			return
		}

		// If the parent (grp) has any subgroups, display their descriptions as
		// subsection headers similar to the output of --help.
		if group.ShortDescription != "" && len(grp.groups) > 0 {
			fmt.Fprintf(wr, ".SS %s\n", group.ShortDescription)

			if group.LongDescription != "" {
				formatForMan(wr, group.LongDescription)
				fmt.Fprintln(wr, "")
			}
		}

		for _, opt := range group.options {
			if !opt.canCli() || opt.Hidden {
				continue
			}

			fmt.Fprintln(wr, ".TP")
			fmt.Fprintf(wr, "\\fB")

			if opt.ShortName != 0 {
				fmt.Fprintf(wr, "\\fB\\-%c\\fR", opt.ShortName)
			}

			if len(opt.LongName) != 0 {
				if opt.ShortName != 0 {
					fmt.Fprintf(wr, ", ")
				}

				fmt.Fprintf(wr, "\\fB\\-\\-%s\\fR", manQuote(opt.LongNameWithNamespace()))
			}

			if len(opt.ValueName) != 0 || opt.OptionalArgument {
				if opt.OptionalArgument {
					fmt.Fprintf(wr, " [\\fI%s=%s\\fR]", manQuote(opt.ValueName), manQuote(strings.Join(quoteV(opt.OptionalValue), ", ")))
				} else {
					fmt.Fprintf(wr, " \\fI%s\\fR", manQuote(opt.ValueName))
				}
			}

			if len(opt.Default) != 0 {
				fmt.Fprintf(wr, " <default: \\fI%s\\fR>", manQuote(strings.Join(quoteV(opt.Default), ", ")))
			} else if len(opt.EnvDefaultKey) != 0 {
				if runtime.GOOS == "windows" {
					fmt.Fprintf(wr, " <default: \\fI%%%s%%\\fR>", manQuote(opt.EnvDefaultKey))
				} else {
					fmt.Fprintf(wr, " <default: \\fI$%s\\fR>", manQuote(opt.EnvDefaultKey))
				}
			}

			if opt.Required {
				fmt.Fprintf(wr, " (\\fIrequired\\fR)")
			}

			fmt.Fprintln(wr, "\\fP")

			if len(opt.Description) != 0 {
				formatForMan(wr, opt.Description)
				fmt.Fprintln(wr, "")
			}
		}
	})
}

func writeManPageSubcommands(wr io.Writer, name string, root *Command) {
	commands := root.sortedVisibleCommands()

	for _, c := range commands {
		var nn string

		if c.Hidden {
			continue
		}

		if len(name) != 0 {
			nn = name + " " + c.Name
		} else {
			nn = c.Name
		}

		writeManPageCommand(wr, nn, root, c)
	}
}

func writeManPageCommand(wr io.Writer, name string, root *Command, command *Command) {
	fmt.Fprintf(wr, ".SS %s\n", name)
	fmt.Fprintln(wr, command.ShortDescription)

	if len(command.LongDescription) > 0 {
		fmt.Fprintln(wr, "")

		cmdstart := fmt.Sprintf("The %s command", manQuote(command.Name))

		if strings.HasPrefix(command.LongDescription, cmdstart) {
			fmt.Fprintf(wr, "The \\fI%s\\fP command", manQuote(command.Name))

			formatForMan(wr, command.LongDescription[len(cmdstart):])
			fmt.Fprintln(wr, "")
		} else {
			formatForMan(wr, command.LongDescription)
			fmt.Fprintln(wr, "")
		}
	}

	var usage string
	if us, ok := command.data.(Usage); ok {
		usage = us.Usage()
	} else if command.hasCliOptions() {
		usage = fmt.Sprintf("[%s-OPTIONS]", command.Name)
	}

	var pre string
	if root.hasCliOptions() {
		pre = fmt.Sprintf("%s [OPTIONS] %s", root.Name, command.Name)
	} else {
		pre = fmt.Sprintf("%s %s", root.Name, command.Name)
	}

	if len(usage) > 0 {
		fmt.Fprintf(wr, "\n\\fBUsage\\fP: %s %s\n.TP\n", manQuote(pre), manQuote(usage))
	}

	if len(command.Aliases) > 0 {
		fmt.Fprintf(wr, "\n\\fBAliases\\fP: %s\n\n", manQuote(strings.Join(command.Aliases, ", ")))
	}

	writeManPageOptions(wr, command.Group)
	writeManPageSubcommands(wr, name, command)
}

// WriteManPage writes a basic man page in groff format to the specified
// writer.
func (p *Parser) WriteManPage(wr io.Writer) {
	t := time.Now()

	fmt.Fprintf(wr, ".TH %s 1 \"%s\"\n", manQuote(p.Name), t.Format("2 January 2006"))
	fmt.Fprintln(wr, ".SH NAME")
	fmt.Fprintf(wr, "%s \\- %s\n", manQuote(p.Name), manQuote(p.ShortDescription))
	fmt.Fprintln(wr, ".SH SYNOPSIS")

	usage := p.Usage

	if len(usage) == 0 {
		usage = "[OPTIONS]"
	}

	fmt.Fprintf(wr, "\\fB%s\\fP %s\n", manQuote(p.Name), manQuote(usage))
	fmt.Fprintln(wr, ".SH DESCRIPTION")

	formatForMan(wr, p.LongDescription)
	fmt.Fprintln(wr, "")

	fmt.Fprintln(wr, ".SH OPTIONS")

	writeManPageOptions(wr, p.Command.Group)

	if len(p.visibleCommands()) > 0 {
		fmt.Fprintln(wr, ".SH COMMANDS")

		writeManPageSubcommands(wr, "", p.Command)
	}
}
