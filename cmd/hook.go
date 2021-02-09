// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli"
)

const (
	hookBatchSize = 30
)

var (
	// CmdHook represents the available hooks sub-command.
	CmdHook = cli.Command{
		Name:        "hook",
		Usage:       "Delegate commands to corresponding Git hooks",
		Description: "This should only be called by Git",
		Subcommands: []cli.Command{
			subcmdHookPreReceive,
			subcmdHookUpdate,
			subcmdHookPostReceive,
			subcmdHookProcReceive,
		},
	}

	subcmdHookPreReceive = cli.Command{
		Name:        "pre-receive",
		Usage:       "Delegate pre-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPreReceive,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subcmdHookUpdate = cli.Command{
		Name:        "update",
		Usage:       "Delegate update Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookUpdate,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subcmdHookPostReceive = cli.Command{
		Name:        "post-receive",
		Usage:       "Delegate post-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPostReceive,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	// Note: new hook since git 2.29
	subcmdHookProcReceive = cli.Command{
		Name:        "proc-receive",
		Usage:       "Delegate proc-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookProcReceive,
		Flags: []cli.Flag{
			cli.BoolFlag{
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
	stopped := util.StopTimer(d.timer)
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
	if os.Getenv(models.EnvIsInternal) == "true" {
		return nil
	}

	setup("hooks/pre-receive.log", c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		} else {
			return nil
		}
	}

	// the environment setted on serv command
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	username := os.Getenv(models.EnvRepoUsername)
	reponame := os.Getenv(models.EnvRepoName)
	userID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	prID, _ := strconv.ParseInt(os.Getenv(models.EnvPRID), 10, 64)
	isDeployKey, _ := strconv.ParseBool(os.Getenv(models.EnvIsDeployKey))

	hookOptions := private.HookOptions{
		UserID:                          userID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		GitPushOptions:                  pushOptions(),
		ProtectedBranchID:               prID,
		IsDeployKey:                     isDeployKey,
	}

	scanner := bufio.NewScanner(os.Stdin)

	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]string, hookBatchSize)
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
		refFullName := string(fields[2])
		total++
		lastline++

		// If the ref is a branch, check if it's protected
		if strings.HasPrefix(refFullName, git.BranchPrefix) {
			oldCommitIDs[count] = oldCommitID
			newCommitIDs[count] = newCommitID
			refFullNames[count] = refFullName
			count++
			fmt.Fprintf(out, "*")

			if count >= hookBatchSize {
				fmt.Fprintf(out, " Checking %d branches\n", count)

				hookOptions.OldCommitIDs = oldCommitIDs
				hookOptions.NewCommitIDs = newCommitIDs
				hookOptions.RefFullNames = refFullNames
				statusCode, msg := private.HookPreReceive(username, reponame, hookOptions)
				switch statusCode {
				case http.StatusOK:
					// no-op
				case http.StatusInternalServerError:
					fail("Internal Server Error", msg)
				default:
					fail(msg, "")
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

		fmt.Fprintf(out, " Checking %d branches\n", count)

		statusCode, msg := private.HookPreReceive(username, reponame, hookOptions)
		switch statusCode {
		case http.StatusInternalServerError:
			fail("Internal Server Error", msg)
		case http.StatusForbidden:
			fail(msg, "")
		}
	} else if lastline > 0 {
		fmt.Fprintf(out, "\n")
		lastline = 0
	}

	fmt.Fprintf(out, "Checked %d references in total\n", total)
	return nil
}

func runHookUpdate(c *cli.Context) error {
	// Update is empty and is kept only for backwards compatibility
	return nil
}

func runHookPostReceive(c *cli.Context) error {
	// First of all run update-server-info no matter what
	if _, err := git.NewCommand("update-server-info").Run(); err != nil {
		return fmt.Errorf("Failed to call 'git update-server-info': %v", err)
	}

	// Now if we're an internal don't do anything else
	if os.Getenv(models.EnvIsInternal) == "true" {
		return nil
	}

	setup("hooks/post-receive.log", c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		} else {
			return nil
		}
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

	// the environment setted on serv command
	repoUser := os.Getenv(models.EnvRepoUsername)
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	repoName := os.Getenv(models.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	pusherName := os.Getenv(models.EnvPusherName)

	hookOptions := private.HookOptions{
		UserName:                        pusherName,
		UserID:                          pusherID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		GitPushOptions:                  pushOptions(),
	}
	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]string, hookBatchSize)
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
		refFullNames[count] = string(fields[2])
		if refFullNames[count] == git.BranchPrefix+"master" && newCommitIDs[count] != git.EmptySHA && count == total {
			masterPushed = true
		}
		count++
		total++

		if count >= hookBatchSize {
			fmt.Fprintf(out, " Processing %d references\n", count)
			hookOptions.OldCommitIDs = oldCommitIDs
			hookOptions.NewCommitIDs = newCommitIDs
			hookOptions.RefFullNames = refFullNames
			resp, err := private.HookPostReceive(repoUser, repoName, hookOptions)
			if resp == nil {
				_ = dWriter.Close()
				hookPrintResults(results)
				fail("Internal Server Error", err)
			}
			wasEmpty = wasEmpty || resp.RepoWasEmpty
			results = append(results, resp.Results...)
			count = 0
		}
	}

	if count == 0 {
		if wasEmpty && masterPushed {
			// We need to tell the repo to reset the default branch to master
			err := private.SetDefaultBranch(repoUser, repoName, "master")
			if err != nil {
				fail("Internal Server Error", "SetDefaultBranch failed with Error: %v", err)
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

	resp, err := private.HookPostReceive(repoUser, repoName, hookOptions)
	if resp == nil {
		_ = dWriter.Close()
		hookPrintResults(results)
		fail("Internal Server Error", err)
	}
	wasEmpty = wasEmpty || resp.RepoWasEmpty
	results = append(results, resp.Results...)

	fmt.Fprintf(out, "Processed %d references in total\n", total)

	if wasEmpty && masterPushed {
		// We need to tell the repo to reset the default branch to master
		err := private.SetDefaultBranch(repoUser, repoName, "master")
		if err != nil {
			fail("Internal Server Error", "SetDefaultBranch failed with Error: %v", err)
		}
	}
	_ = dWriter.Close()
	hookPrintResults(results)

	return nil
}

func hookPrintResults(results []private.HookPostReceiveBranchResult) {
	for _, res := range results {
		if !res.Message {
			continue
		}

		fmt.Fprintln(os.Stderr, "")
		if res.Create {
			fmt.Fprintf(os.Stderr, "Create a new pull request for '%s':\n", res.Branch)
			fmt.Fprintf(os.Stderr, "  %s\n", res.URL)
		} else {
			fmt.Fprint(os.Stderr, "Visit the existing pull request:\n")
			fmt.Fprintf(os.Stderr, "  %s\n", res.URL)
		}
		fmt.Fprintln(os.Stderr, "")
		os.Stderr.Sync()
	}
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
	setup("hooks/proc-receive.log", c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		} else {
			return nil
		}
	}

	if git.CheckGitVersionAtLeast("2.29") != nil {
		fail("Internal Server Error", "git not support proc-receive.")
	}

	reader := bufio.NewReader(os.Stdin)
	repoUser := os.Getenv(models.EnvRepoUsername)
	repoName := os.Getenv(models.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	pusherName := os.Getenv(models.EnvPusherName)

	// 1. Version and features negotiation.
	// S: PKT-LINE(version=1\0push-options atomic...)
	// S: flush-pkt
	// H: PKT-LINE(version=1\0push-options...)
	// H: flush-pkt

	rs := readPktLine(reader, pktLineTypeData)

	const VersionHead string = "version=1"

	hasPushOptions := false
	response := []byte(VersionHead)
	var requestOptions []string

	for i := range rs.Data {
		if rs.Data[i] == byte(0) {
			if string(rs.Data[0:i]) != VersionHead {
				fail("Internal Server Error", "Received an not supported version: %s", string(rs.Data[0:i]))
			}
			requestOptions = strings.Split(string(rs.Data[i+1:]), " ")
			break
		}
	}

	for _, option := range requestOptions {
		if strings.HasPrefix(option, "push-options") {
			response = append(response, byte(0))
			response = append(response, []byte("push-options")...)
			hasPushOptions = true
		}
	}
	response = append(response, []byte("\n")...)

	rs = readPktLine(reader, pktLineTypeFlush)

	writePktLine(os.Stdout, pktLineTypeData, response)
	writePktLine(os.Stdout, pktLineTypeFlush, nil)

	// 2. receive commands from server.
	// S: PKT-LINE(<old-oid> <new-oid> <ref>)
	// S: ... ...
	// S: flush-pkt
	// # [receive push-options]
	// S: PKT-LINE(push-option)
	// S: ... ...
	// S: flush-pkt
	hookOptions := private.HookOptions{
		UserName: pusherName,
		UserID:   pusherID,
	}
	hookOptions.OldCommitIDs = make([]string, 0, hookBatchSize)
	hookOptions.NewCommitIDs = make([]string, 0, hookBatchSize)
	hookOptions.RefFullNames = make([]string, 0, hookBatchSize)

	for {
		// note: pktLineTypeUnknow means pktLineTypeFlush and pktLineTypeData all allowed
		rs = readPktLine(reader, pktLineTypeUnknow)

		if rs.Type == pktLineTypeFlush {
			break
		}
		t := strings.SplitN(string(rs.Data), " ", 3)
		if len(t) != 3 {
			continue
		}
		hookOptions.OldCommitIDs = append(hookOptions.OldCommitIDs, t[0])
		hookOptions.NewCommitIDs = append(hookOptions.NewCommitIDs, t[1])
		hookOptions.RefFullNames = append(hookOptions.RefFullNames, t[2])
	}

	hookOptions.GitPushOptions = make(map[string]string)

	if hasPushOptions {
		for {
			rs = readPktLine(reader, pktLineTypeUnknow)

			if rs.Type == pktLineTypeFlush {
				break
			}

			kv := strings.SplitN(string(rs.Data), "=", 2)
			if len(kv) == 2 {
				hookOptions.GitPushOptions[kv[0]] = kv[1]
			}
		}
	}

	// 3. run hook
	resp, err := private.HookProcReceive(repoUser, repoName, hookOptions)
	if err != nil {
		fail("Internal Server Error", "run proc-receive hook failed :%v", err)
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

	for _, rs := range resp.Results {
		if len(rs.Err) > 0 {
			writePktLine(os.Stdout, pktLineTypeData, []byte("ng "+rs.OrignRef+" "+rs.Err))
			continue
		}

		writePktLine(os.Stdout, pktLineTypeData, []byte("ok "+rs.OrignRef))
		writePktLine(os.Stdout, pktLineTypeData, []byte("option refname "+rs.Ref))
		if rs.OldOID != git.EmptySHA {
			writePktLine(os.Stdout, pktLineTypeData, []byte("option old-oid "+rs.OldOID))
		}
		writePktLine(os.Stdout, pktLineTypeData, []byte("option new-oid "+rs.NewOID))
	}
	writePktLine(os.Stdout, pktLineTypeFlush, nil)

	return nil
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

func readPktLine(in *bufio.Reader, requestType pktLineType) (r *gitPktLine) {
	var err error

	// read prefix
	lengthBytes := make([]byte, 4)
	for i := 0; i < 4; i++ {
		lengthBytes[i], err = in.ReadByte()
		if err != nil {
			fail("Internal Server Error", "Pkt-Line: read stdin failed : %v", err)
		}
	}

	r = new(gitPktLine)
	r.Length, err = strconv.ParseUint(string(lengthBytes), 16, 32)
	if err != nil {
		fail("Internal Server Error", "Pkt-Line format is wrong :%v", err)
	}

	if r.Length == 0 {
		if requestType == pktLineTypeData {
			fail("Internal Server Error", "Pkt-Line format is wrong")
		}
		r.Type = pktLineTypeFlush
		return r
	}

	if r.Length <= 4 || r.Length > 65520 || requestType == pktLineTypeFlush {
		fail("Internal Server Error", "Pkt-Line format is wrong")
	}

	r.Data = make([]byte, r.Length-4)
	for i := range r.Data {
		r.Data[i], err = in.ReadByte()
		if err != nil {
			fail("Internal Server Error", "Pkt-Line: read stdin failed : %v", err)
		}
	}

	r.Type = pktLineTypeData

	return r
}

func writePktLine(out io.Writer, typ pktLineType, data []byte) {
	if typ == pktLineTypeFlush {
		l, err := out.Write([]byte("0000"))
		if err != nil {
			fail("Internal Server Error", "Pkt-Line response failed: %v", err)
		}
		if l != 4 {
			fail("Internal Server Error", "Pkt-Line response failed: %v", err)
		}
	}

	if typ != pktLineTypeData {
		return
	}

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
	if err != nil {
		fail("Internal Server Error", "Pkt-Line response failed: %v", err)
	}
	if 4 != lr {
		fail("Internal Server Error", "Pkt-Line response failed: %v", err)
	}

	lr, err = out.Write(data)
	if err != nil {
		fail("Internal Server Error", "Pkt-Line response failed: %v", err)
	}
	if int(length-4) != lr {
		fail("Internal Server Error", "Pkt-Line response failed: %v", err)
	}
}
