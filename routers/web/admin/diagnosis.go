// Copyright 2023 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"archive/zip"
	"fmt"
	"runtime/pprof"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httplib"
)

func MonitorDiagnosis(ctx *context.Context) {
	seconds := ctx.FormInt64("seconds")
	if seconds <= 5 {
		seconds = 5
	}
	if seconds > 300 {
		seconds = 300
	}

	httplib.ServeSetHeaders(ctx.Resp, &httplib.ServeHeaderOptions{
		ContentType: "application/zip",
		Disposition: "attachment",
		Filename:    fmt.Sprintf("gitea-diagnosis-%s.zip", time.Now().Format("20060102-150405")),
	})

	zipWriter := zip.NewWriter(ctx.Resp)
	defer zipWriter.Close()

	f, err := zipWriter.CreateHeader(&zip.FileHeader{Name: "goroutine-before.txt", Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		ctx.ServerError("Failed to create zip file", err)
		return
	}
	_ = pprof.Lookup("goroutine").WriteTo(f, 1)

	f, err = zipWriter.CreateHeader(&zip.FileHeader{Name: "cpu-profile.dat", Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		ctx.ServerError("Failed to create zip file", err)
		return
	}

	err = pprof.StartCPUProfile(f)
	if err == nil {
		time.Sleep(time.Duration(seconds) * time.Second)
		pprof.StopCPUProfile()
	} else {
		_, _ = f.Write([]byte(err.Error()))
	}

	f, err = zipWriter.CreateHeader(&zip.FileHeader{Name: "goroutine-after.txt", Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		ctx.ServerError("Failed to create zip file", err)
		return
	}
	_ = pprof.Lookup("goroutine").WriteTo(f, 1)
}
