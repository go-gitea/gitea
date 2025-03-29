// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v2"
)

const (
	hookBatchSize = 30
)

var (
	// CmdHook represents the available hooks sub-command.
	CmdHook = &cli.Command{
		Name:        "hook",
		Usage:       "(internal) Should only be called by Git",
		Description: "Delegate commands to corresponding Git hooks",
		Before:      PrepareConsoleLoggerLevel(log.FATAL),
		Subcommands: []*cli.Command{
			subcmdHookPreReceive,
			subcmdHookUpdate,
			subcmdHookPostReceive,
			subcmdHookProcReceive,
		},
	}

	subcmdHookPreReceive = &cli.Command{
		Name:        "pre-receive",
		Usage:       "Delegate pre-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPreReceive,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subcmdHookUpdate = &cli.Command{
		Name:        "update",
		Usage:       "Delegate update Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookUpdate,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subcmdHookPostReceive = &cli.Command{
		Name:        "post-receive",
		Usage:       "Delegate post-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPostReceive,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	// Note: new hook since git 2.29
	subcmdHookProcReceive = &cli.Command{
		Name:        "proc-receive",
		Usage:       "Delegate proc-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookProcReceive,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "debug",
			},
		},
	}
)

type delayWriter struct {
	internal io.Writer
	buf      *bytes.Buffer
	timer    *time.Timer
}

func newDelayWriter(internal io.Writer, delay time.Duration) *delayWriter {
	timer := time.NewTimer(delay)
	return &delayWriter{
		internal: internal,
		buf:      &bytes.Buffer{},
		timer:    timer,
	}
}

func (d *delayWriter) Write(p []byte) (n int, err error) {
	if d.buf != nil {
		select {
		case <-d.timer.C:
			_, err := d.internal.Write(d.buf.Bytes())
			if err != nil {
				return 0, err
			}
			d.buf = nil
			return d.internal.Write(p)
		default:
			return d.buf.Write(p)
		}
	}
	return d.internal.Write(p)
}

func (d *delayWriter) WriteString(s string) (n int, err error) {
	if d.buf != nil {
		select {
		case <-d.timer.C:
			_, err := d.internal.Write(d.buf.Bytes())
			if err != nil {
				return 0, err
			}
			d.buf = nil
			return d.internal.Write([]byte(s))
		default:
			return d.buf.WriteString(s)
		}
	}
	return d.internal.Write([]byte(s))
}

func (d *delayWriter) Close() error {
	if d == nil {
		return nil
	}
	stopped := d.timer.Stop()
	if stopped || d.buf == nil {
		return nil
	}
	_, err := d.internal.Write(d.buf.Bytes())
	d.buf = nil
	return err
}

type nilWriter struct{}

func (n *nilWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n *nilWriter) WriteString(s string) (int, error) {
	return len(s), nil
}

func runHookPreReceive(c *cli.Context) error {
	if isInternal, _ := strconv.ParseBool(os.Getenv(repo_module.EnvIsInternal)); isInternal {
		return nil
	}
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			return fail(ctx, `Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		}
		return nil
	}

	// the environment is set by serv command
	isWiki, _ := strconv.ParseBool(os.Getenv(repo_module.EnvRepoIsWiki))
	username := os.Getenv(repo_module.EnvRepoUsername)
	reponame := os.Getenv(repo_module.EnvRepoName)
	userID, _ := strconv.ParseInt(os.Getenv(repo_module.EnvPusherID), 10, 64)
	prID, _ := strconv.ParseInt(os.Getenv(repo_module.EnvPRID), 10, 64)
	deployKeyID, _ := strconv.ParseInt(os.Getenv(repo_module.EnvDeployKeyID), 10, 64)
	actionPerm, _ := strconv.ParseInt(os.Getenv(repo_module.EnvActionPerm), 10, 64)

	hookOptions := private.HookOptions{
		UserID:                          userID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		GitPushOptions:                  pushOptions(),
		PullRequestID:                   prID,
		DeployKeyID:                     deployKeyID,
		ActionPerm:                      int(actionPerm),
	}

	scanner := bufio.NewScanner(os.Stdin)

	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]git.RefName, hookBatchSize)
	count := 0
	total := 0
	lastline := 0

	var out io.Writer
	out = &nilWriter{}
	if setting.Git.VerbosePush {
		if setting.Git.VerbosePushDelay > 0 {
			dWriter := newDelayWriter(os.Stdout, setting.Git.VerbosePushDelay)
			defer dWriter.Close()
			out = dWriter
		} else {
			out = os.Stdout
		}
	}

	supportProcReceive := git.DefaultFeatures().SupportProcReceive

	for scanner.Scan() {
		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		oldCommitID := string(fields[0])
		newCommitID := string(fields[1])
		refFullName := git.RefName(fields[2])
		total++
		lastline++

		// If the ref is a branch or tag, check if it's protected
		// if supportProcReceive all ref should be checked because
		// permission check was delayed
		if supportProcReceive || refFullName.IsBranch() || refFullName.IsTag() {
			oldCommitIDs[count] = oldCommitID
			newCommitIDs[count] = newCommitID
			refFullNames[count] = refFullName
			count++
			fmt.Fprintf(out, "*")

			if count >= hookBatchSize {
				fmt.Fprintf(out, " Checking %d references\n", count)

				hookOptions.OldCommitIDs = oldCommitIDs
				hookOptions.NewCommitIDs = newCommitIDs
				hookOptions.RefFullNames = refFullNames
				extra := private.HookPreReceive(ctx, username, reponame, hookOptions)
				if extra.HasError() {
					return fail(ctx, extra.UserMsg, "HookPreReceive(batch) failed: %v", extra.Error)
				}
				count = 0
				lastline = 0
			}
		} else {
			fmt.Fprintf(out, ".")
		}
		if lastline >= hookBatchSize {
			fmt.Fprintf(out, "\n")
			lastline = 0
		}
	}

	if count > 0 {
		hookOptions.OldCommitIDs = oldCommitIDs[:count]
		hookOptions.NewCommitIDs = newCommitIDs[:count]
		hookOptions.RefFullNames = refFullNames[:count]

		fmt.Fprintf(out, " Checking %d references\n", count)

		extra := private.HookPreReceive(ctx, username, reponame, hookOptions)
		if extra.HasError() {
			return fail(ctx, extra.UserMsg, "HookPreReceive(last) failed: %v", extra.Error)
		}
	} else if lastline > 0 {
		fmt.Fprintf(out, "\n")
	}

	fmt.Fprintf(out, "Checked %d references in total\n", total)
	return nil
}

// runHookUpdate avoid to do heavy operations on update hook because it will be
// invoked for every ref update which does not like pre-receive and post-receive
func runHookUpdate(c *cli.Context) error {
	if isInternal, _ := strconv.ParseBool(os.Getenv(repo_module.EnvIsInternal)); isInternal {
		return nil
	}

	// Update is empty and is kept only for backwards compatibility
	if len(os.Args) < 3 {
		return nil
	}
	refName := git.RefName(os.Args[len(os.Args)-3])
	if refName.IsPull() {
		// ignore update to refs/pull/xxx/head, so we don't need to output any information
		os.Exit(1)
	}
	return nil
}

func runHookPostReceive(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))

	// First of all run update-server-info no matter what
	if _, _, err := git.NewCommand("update-server-info").RunStdString(ctx, nil); err != nil {
		return fmt.Errorf("Failed to call 'git update-server-info': %w", err)
	}

	// Now if we're an internal don't do anything else
	if isInternal, _ := strconv.ParseBool(os.Getenv(repo_module.EnvIsInternal)); isInternal {
		return nil
	}

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			return fail(ctx, `Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		}
		return nil
	}

	var out io.Writer
	var dWriter *delayWriter
	out = &nilWriter{}
	if setting.Git.VerbosePush {
		if setting.Git.VerbosePushDelay > 0 {
			dWriter = newDelayWriter(os.Stdout, setting.Git.VerbosePushDelay)
			defer dWriter.Close()
			out = dWriter
		} else {
			out = os.Stdout
		}
	}

	// the environment is set by serv command
	repoUser := os.Getenv(repo_module.EnvRepoUsername)
	isWiki, _ := strconv.ParseBool(os.Getenv(repo_module.EnvRepoIsWiki))
	repoName := os.Getenv(repo_module.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(repo_module.EnvPusherID), 10, 64)
	prID, _ := strconv.ParseInt(os.Getenv(repo_module.EnvPRID), 10, 64)
	pusherName := os.Getenv(repo_module.EnvPusherName)

	hookOptions := private.HookOptions{
		UserName:                        pusherName,
		UserID:                          pusherID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		GitPushOptions:                  pushOptions(),
		PullRequestID:                   prID,
		PushTrigger:                     repo_module.PushTrigger(os.Getenv(repo_module.EnvPushTrigger)),
	}
	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]git.RefName, hookBatchSize)
	count := 0
	total := 0
	wasEmpty := false
	masterPushed := false
	results := make([]private.HookPostReceiveBranchResult, 0)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		fmt.Fprintf(out, ".")
		oldCommitIDs[count] = string(fields[0])
		newCommitIDs[count] = string(fields[1])
		refFullNames[count] = git.RefName(fields[2])

		commitID, _ := git.NewIDFromString(newCommitIDs[count])
		if refFullNames[count] == git.BranchPrefix+"master" && !commitID.IsZero() && count == total {
			masterPushed = true
		}
		count++
		total++

		if count >= hookBatchSize {
			fmt.Fprintf(out, " Processing %d references\n", count)
			hookOptions.OldCommitIDs = oldCommitIDs
			hookOptions.NewCommitIDs = newCommitIDs
			hookOptions.RefFullNames = refFullNames
			resp, extra := private.HookPostReceive(ctx, repoUser, repoName, hookOptions)
			if extra.HasError() {
				_ = dWriter.Close()
				hookPrintResults(results)
				return fail(ctx, extra.UserMsg, "HookPostReceive failed: %v", extra.Error)
			}
			wasEmpty = wasEmpty || resp.RepoWasEmpty
			results = append(results, resp.Results...)
			count = 0
		}
	}

	if count == 0 {
		if wasEmpty && masterPushed {
			// We need to tell the repo to reset the default branch to master
			extra := private.SetDefaultBranch(ctx, repoUser, repoName, "master")
			if extra.HasError() {
				return fail(ctx, extra.UserMsg, "SetDefaultBranch failed: %v", extra.Error)
			}
		}
		fmt.Fprintf(out, "Processed %d references in total\n", total)

		_ = dWriter.Close()
		hookPrintResults(results)
		return nil
	}

	hookOptions.OldCommitIDs = oldCommitIDs[:count]
	hookOptions.NewCommitIDs = newCommitIDs[:count]
	hookOptions.RefFullNames = refFullNames[:count]

	fmt.Fprintf(out, " Processing %d references\n", count)

	resp, extra := private.HookPostReceive(ctx, repoUser, repoName, hookOptions)
	if resp == nil {
		_ = dWriter.Close()
		hookPrintResults(results)
		return fail(ctx, extra.UserMsg, "HookPostReceive failed: %v", extra.Error)
	}
	wasEmpty = wasEmpty || resp.RepoWasEmpty
	results = append(results, resp.Results...)

	fmt.Fprintf(out, "Processed %d references in total\n", total)

	if wasEmpty && masterPushed {
		// We need to tell the repo to reset the default branch to master
		extra := private.SetDefaultBranch(ctx, repoUser, repoName, "master")
		if extra.HasError() {
			return fail(ctx, extra.UserMsg, "SetDefaultBranch failed: %v", extra.Error)
		}
	}
	_ = dWriter.Close()
	hookPrintResults(results)

	return nil
}

func hookPrintResults(results []private.HookPostReceiveBranchResult) {
	for _, res := range results {
		hookPrintResult(res.Message, res.Create, res.Branch, res.URL)
	}
}

func hookPrintResult(output, isCreate bool, branch, url string) {
	if !output {
		return
	}
	fmt.Fprintln(os.Stderr, "")
	if isCreate {
		fmt.Fprintf(os.Stderr, "Create a new pull request for '%s':\n", branch)
		fmt.Fprintf(os.Stderr, "  %s\n", url)
	} else {
		fmt.Fprint(os.Stderr, "Visit the existing pull request:\n")
		fmt.Fprintf(os.Stderr, "  %s\n", url)
	}
	fmt.Fprintln(os.Stderr, "")
	_ = os.Stderr.Sync()
}

func pushOptions() map[string]string {
	opts := make(map[string]string)
	if pushCount, err := strconv.Atoi(os.Getenv(private.GitPushOptionCount)); err == nil {
		for idx := 0; idx < pushCount; idx++ {
			opt := os.Getenv(fmt.Sprintf("GIT_PUSH_OPTION_%d", idx))
			kv := strings.SplitN(opt, "=", 2)
			if len(kv) == 2 {
				opts[kv[0]] = kv[1]
			}
		}
	}
	return opts
}

func runHookProcReceive(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			return fail(ctx, `Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		}
		return nil
	}

	if !git.DefaultFeatures().SupportProcReceive {
		return fail(ctx, "No proc-receive support", "current git version doesn't support proc-receive.")
	}

	reader := bufio.NewReader(os.Stdin)
	repoUser := os.Getenv(repo_module.EnvRepoUsername)
	repoName := os.Getenv(repo_module.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(repo_module.EnvPusherID), 10, 64)
	pusherName := os.Getenv(repo_module.EnvPusherName)

	// 1. Version and features negotiation.
	// S: PKT-LINE(version=1\0push-options atomic...) / PKT-LINE(version=1\n)
	// S: flush-pkt
	// H: PKT-LINE(version=1\0push-options...)
	// H: flush-pkt

	rs, err := readPktLine(ctx, reader, pktLineTypeData)
	if err != nil {
		return err
	}

	const VersionHead string = "version=1"

	var (
		hasPushOptions bool
		response       = []byte(VersionHead)
		requestOptions []string
	)

	index := bytes.IndexByte(rs.Data, byte(0))
	if index >= len(rs.Data) {
		return fail(ctx, "Protocol: format error", "pkt-line: format error %s", rs.Data)
	}

	if index < 0 {
		if len(rs.Data) == 10 && rs.Data[9] == '\n' {
			index = 9
		} else {
			return fail(ctx, "Protocol: format error", "pkt-line: format error %s", rs.Data)
		}
	}

	if string(rs.Data[0:index]) != VersionHead {
		return fail(ctx, "Protocol: version error", "Received unsupported version: %s", string(rs.Data[0:index]))
	}
	requestOptions = strings.Split(string(rs.Data[index+1:]), " ")

	for _, option := range requestOptions {
		if strings.HasPrefix(option, "push-options") {
			response = append(response, byte(0))
			response = append(response, []byte("push-options")...)
			hasPushOptions = true
		}
	}
	response = append(response, '\n')

	_, err = readPktLine(ctx, reader, pktLineTypeFlush)
	if err != nil {
		return err
	}

	err = writeDataPktLine(ctx, os.Stdout, response)
	if err != nil {
		return err
	}

	err = writeFlushPktLine(ctx, os.Stdout)
	if err != nil {
		return err
	}

	// 2. receive commands from server.
	// S: PKT-LINE(<old-oid> <new-oid> <ref>)
	// S: ... ...
	// S: flush-pkt
	// # [receive push-options]
	// S: PKT-LINE(push-option)
	// S: ... ...
	// S: flush-pkt
	hookOptions := private.HookOptions{
		UserName:       pusherName,
		UserID:         pusherID,
		GitPushOptions: make(map[string]string),
	}
	hookOptions.OldCommitIDs = make([]string, 0, hookBatchSize)
	hookOptions.NewCommitIDs = make([]string, 0, hookBatchSize)
	hookOptions.RefFullNames = make([]git.RefName, 0, hookBatchSize)

	for {
		// note: pktLineTypeUnknow means pktLineTypeFlush and pktLineTypeData all allowed
		rs, err = readPktLine(ctx, reader, pktLineTypeUnknow)
		if err != nil {
			return err
		}

		if rs.Type == pktLineTypeFlush {
			break
		}
		t := strings.SplitN(string(rs.Data), " ", 3)
		if len(t) != 3 {
			continue
		}
		hookOptions.OldCommitIDs = append(hookOptions.OldCommitIDs, t[0])
		hookOptions.NewCommitIDs = append(hookOptions.NewCommitIDs, t[1])
		hookOptions.RefFullNames = append(hookOptions.RefFullNames, git.RefName(t[2]))
	}

	if hasPushOptions {
		for {
			rs, err = readPktLine(ctx, reader, pktLineTypeUnknow)
			if err != nil {
				return err
			}

			if rs.Type == pktLineTypeFlush {
				break
			}
			hookOptions.GitPushOptions.AddFromKeyValue(string(rs.Data))
		}
	}

	// 3. run hook
	resp, extra := private.HookProcReceive(ctx, repoUser, repoName, hookOptions)
	if extra.HasError() {
		return fail(ctx, extra.UserMsg, "HookProcReceive failed: %v", extra.Error)
	}

	// 4. response result to service
	// # a. OK, but has an alternate reference.  The alternate reference name
	// # and other status can be given in option directives.
	// H: PKT-LINE(ok <ref>)
	// H: PKT-LINE(option refname <refname>)
	// H: PKT-LINE(option old-oid <old-oid>)
	// H: PKT-LINE(option new-oid <new-oid>)
	// H: PKT-LINE(option forced-update)
	// H: ... ...
	// H: flush-pkt
	// # b. NO, I reject it.
	// H: PKT-LINE(ng <ref> <reason>)
	// # c. Fall through, let 'receive-pack' to execute it.
	// H: PKT-LINE(ok <ref>)
	// H: PKT-LINE(option fall-through)

	for _, rs := range resp.Results {
		if len(rs.Err) > 0 {
			err = writeDataPktLine(ctx, os.Stdout, []byte("ng "+rs.OriginalRef.String()+" "+rs.Err))
			if err != nil {
				return err
			}
			continue
		}

		if rs.IsNotMatched {
			err = writeDataPktLine(ctx, os.Stdout, []byte("ok "+rs.OriginalRef.String()))
			if err != nil {
				return err
			}
			err = writeDataPktLine(ctx, os.Stdout, []byte("option fall-through"))
			if err != nil {
				return err
			}
			continue
		}

		err = writeDataPktLine(ctx, os.Stdout, []byte("ok "+rs.OriginalRef))
		if err != nil {
			return err
		}
		err = writeDataPktLine(ctx, os.Stdout, []byte("option refname "+rs.Ref))
		if err != nil {
			return err
		}
		commitID, _ := git.NewIDFromString(rs.OldOID)
		if !commitID.IsZero() {
			err = writeDataPktLine(ctx, os.Stdout, []byte("option old-oid "+rs.OldOID))
			if err != nil {
				return err
			}
		}
		err = writeDataPktLine(ctx, os.Stdout, []byte("option new-oid "+rs.NewOID))
		if err != nil {
			return err
		}
		if rs.IsForcePush {
			err = writeDataPktLine(ctx, os.Stdout, []byte("option forced-update"))
			if err != nil {
				return err
			}
		}
	}
	err = writeFlushPktLine(ctx, os.Stdout)

	if err == nil {
		for _, res := range resp.Results {
			hookPrintResult(res.ShouldShowMessage, res.IsCreatePR, res.HeadBranch, res.URL)
		}
	}

	return err
}

// git PKT-Line api
// pktLineType message type of pkt-line
type pktLineType int64

const (
	// UnKnow type
	pktLineTypeUnknow pktLineType = 0
	// flush-pkt "0000"
	pktLineTypeFlush pktLineType = iota
	// data line
	pktLineTypeData
)

// gitPktLine pkt-line api
type gitPktLine struct {
	Type   pktLineType
	Length uint64
	Data   []byte
}

func readPktLine(ctx context.Context, in *bufio.Reader, requestType pktLineType) (*gitPktLine, error) {
	var (
		err error
		r   *gitPktLine
	)

	// read prefix
	lengthBytes := make([]byte, 4)
	for i := 0; i < 4; i++ {
		lengthBytes[i], err = in.ReadByte()
		if err != nil {
			return nil, fail(ctx, "Protocol: stdin error", "Pkt-Line: read stdin failed : %v", err)
		}
	}

	r = new(gitPktLine)
	r.Length, err = strconv.ParseUint(string(lengthBytes), 16, 32)
	if err != nil {
		return nil, fail(ctx, "Protocol: format parse error", "Pkt-Line format is wrong :%v", err)
	}

	if r.Length == 0 {
		if requestType == pktLineTypeData {
			return nil, fail(ctx, "Protocol: format data error", "Pkt-Line format is wrong")
		}
		r.Type = pktLineTypeFlush
		return r, nil
	}

	if r.Length <= 4 || r.Length > 65520 || requestType == pktLineTypeFlush {
		return nil, fail(ctx, "Protocol: format length error", "Pkt-Line format is wrong")
	}

	r.Data = make([]byte, r.Length-4)
	for i := range r.Data {
		r.Data[i], err = in.ReadByte()
		if err != nil {
			return nil, fail(ctx, "Protocol: data error", "Pkt-Line: read stdin failed : %v", err)
		}
	}

	r.Type = pktLineTypeData

	return r, nil
}

func writeFlushPktLine(ctx context.Context, out io.Writer) error {
	l, err := out.Write([]byte("0000"))
	if err != nil || l != 4 {
		return fail(ctx, "Protocol: write error", "Pkt-Line response failed: %v", err)
	}
	return nil
}

func writeDataPktLine(ctx context.Context, out io.Writer, data []byte) error {
	hexchar := []byte("0123456789abcdef")
	hex := func(n uint64) byte {
		return hexchar[(n)&15]
	}

	length := uint64(len(data) + 4)
	tmp := make([]byte, 4)
	tmp[0] = hex(length >> 12)
	tmp[1] = hex(length >> 8)
	tmp[2] = hex(length >> 4)
	tmp[3] = hex(length)

	lr, err := out.Write(tmp)
	if err != nil || lr != 4 {
		return fail(ctx, "Protocol: write error", "Pkt-Line response failed: %v", err)
	}

	lr, err = out.Write(data)
	if err != nil || int(length-4) != lr {
		return fail(ctx, "Protocol: write error", "Pkt-Line response failed: %v", err)
	}

	return nil
}
