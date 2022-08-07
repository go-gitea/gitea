package dev

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/dbfs"
	"code.gitea.io/gitea/modules/context"
)

var demoLogWriterOnce sync.Once

func TermDemo(ctx *context.Context) {
	demoLogWriterOnce.Do(func() {
		go func() {
			f, _ := dbfs.OpenFile(db.DefaultContext, "termdemo.log", os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND)
			count := 0
			for {
				count++
				s := fmt.Sprintf("\x1B[1;3;31mDemo Log\x1B[0m, count=%d\r\n", count)
				_, _ = f.Write([]byte(s))
				time.Sleep(time.Second)
			}
		}()
	})

	cmd := ctx.FormString("cmd")
	if cmd == "tail" {
		offset := ctx.FormInt64("offset")
		f, _ := dbfs.OpenFile(db.DefaultContext, "termdemo.log", os.O_RDONLY)
		if offset == -1 {
			_, _ = f.Seek(0, io.SeekEnd)
		} else {
			_, _ = f.Seek(offset, io.SeekStart)
		}
		buf, _ := io.ReadAll(f)
		offset, _ = f.Seek(0, io.SeekCurrent)
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"offset":  offset,
			"content": string(buf),
		})
		return
	}

	ctx.HTML(http.StatusOK, "dev/termdemo")
}
