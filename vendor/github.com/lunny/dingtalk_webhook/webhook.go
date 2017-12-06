// Copyright 2017 Lunny Xiao. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dingtalk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

/*
{
	"msgtype": "text",
	"text": {
		"content": "我就是我, 是不一样的烟火"
	},
	"at": {
		"atMobiles": [
			"156xxxx8827",
			"189xxxx8325"
		],
		"isAtAll": false
	}
}

{
	"msgtype": "link",
	"link": {
		"text": "这个即将发布的新版本，创始人陈航（花名“无招”）称它为“红树林”。
而在此之前，每当面临重大升级，产品经理们都会取一个应景的代号，这一次，为什么是“红树林”？",
		"title": "时代的火车向前开",
		"picUrl": "",
		"messageUrl": "https://mp.weixin.qq.com/s?__biz=MzA4NjMwMTA2Ng==&mid=2650316842&idx=1&sn=60da3ea2b29f1dcc43a7c8e4a7c97a16&scene=2&srcid=09189AnRJEdIiWVaKltFzNTw&from=timeline&isappinstalled=0&key=&ascene=2&uin=&devicetype=android-23&version=26031933&nettype=WIFI"
	}
}

{
	"msgtype": "markdown",
	"markdown": {
		"title":"杭州天气",
		"text": "#### 杭州天气 @156xxxx8827\n" +
				"> 9度，西北风1级，空气良89，相对温度73%\n\n" +
				"> ![screenshot](http://image.jpg)\n"  +
				"> ###### 10点20分发布 [天气](http://www.thinkpage.cn/) \n"
	},
	"at": {
		"atMobiles": [
			"156xxxx8827",
			"189xxxx8325"
		],
		"isAtAll": false
	}
}

{
    "actionCard": {
        "title": "乔布斯 20 年前想打造一间苹果咖啡厅，而它正是 Apple Store 的前身",
        "text": "![screenshot](@lADOpwk3K80C0M0FoA)
 ### 乔布斯 20 年前想打造的苹果咖啡厅
 Apple Store 的设计正从原来满满的科技感走向生活化，而其生活化的走向其实可以追溯到 20 年前苹果一个建立咖啡馆的计划",
        "hideAvatar": "0",
        "btnOrientation": "0",
        "singleTitle" : "阅读全文",
		"singleURL" : "https://www.dingtalk.com/",
		"btns": [
            {
                "title": "内容不错",
                "actionURL": "https://www.dingtalk.com/"
            },
            {
                "title": "不感兴趣",
                "actionURL": "https://www.dingtalk.com/"
            }
        ]
    },
    "msgtype": "actionCard"
}

{
    "feedCard": {
        "links": [
            {
                "title": "时代的火车向前开",
                "messageURL": "https://mp.weixin.qq.com/s?__biz=MzA4NjMwMTA2Ng==&mid=2650316842&idx=1&sn=60da3ea2b29f1dcc43a7c8e4a7c97a16&scene=2&srcid=09189AnRJEdIiWVaKltFzNTw&from=timeline&isappinstalled=0&key=&ascene=2&uin=&devicetype=android-23&version=26031933&nettype=WIFI",
                "picURL": "https://www.dingtalk.com/"
            },
            {
                "title": "时代的火车向前开2",
                "messageURL": "https://mp.weixin.qq.com/s?__biz=MzA4NjMwMTA2Ng==&mid=2650316842&idx=1&sn=60da3ea2b29f1dcc43a7c8e4a7c97a16&scene=2&srcid=09189AnRJEdIiWVaKltFzNTw&from=timeline&isappinstalled=0&key=&ascene=2&uin=&devicetype=android-23&version=26031933&nettype=WIFI",
                "picURL": "https://www.dingtalk.com/"
            }
        ]
    },
    "msgtype": "feedCard"
}
*/

type LinkMsg struct {
	Title      string `json:"title"`
	MessageURL string `json:"messageURL"`
	PicURL     string `json:"picURL"`
}

type ActionCard struct {
	Text           string `json:"text"`
	Title          string `json:"title"`
	HideAvatar     string `json:"hideAvatar"`
	BtnOrientation string `json:"btnOrientation"`
	SingleTitle    string `json:"singleTitle"`
	SingleURL      string `json:"singleURL"`
	Buttons        []struct {
		Title     string `json:"title"`
		ActionURL string `json:"actionURL"`
	} `json:"btns"`
}

// Payload struct
type Payload struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Link struct {
		Text       string `json:"text"`
		Title      string `json:"title"`
		PicURL     string `json:"picUrl"`
		MessageURL string `json:"messageUrl"`
	} `json:"link"`
	Markdown struct {
		Text  string `json:"text"`
		Title string `json:"title"`
	} `json:"markdown"`
	ActionCard ActionCard `json:"actionCard"`
	FeedCard   struct {
		Links []LinkMsg `json:"links"`
	} `json:"feedCard"`
	At struct {
		AtMobiles []string `json:"atMobiles"`
		IsAtAll   bool     `json:"isAtAll"`
	} `json:"at"`
}

type Webhook struct {
	accessToken string
}

func NewWebhook(accessToken string) *Webhook {
	return &Webhook{accessToken}
}

type Response struct {
	ErrorCode    int    `json:"errcode"`
	ErrorMessage string `json:"errmsg"`
}

// SendPayload 发送消息
func (w *Webhook) SendPayload(payload *Payload) error {
	bs, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post("https://oapi.dingtalk.com/robot/send?access_token="+w.accessToken, "application/json", bytes.NewReader(bs))
	if err != nil {
		return err
	}

	bs, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("%d: %s", resp.StatusCode, string(bs))
	}

	var result Response
	err = json.Unmarshal(bs, &result)
	if err != nil {
		return err
	}
	if result.ErrorCode != 0 {
		return fmt.Errorf("%d: %s", result.ErrorCode, result.ErrorMessage)
	}

	return nil
}

// SendTextMsg 发送文本消息
func (w *Webhook) SendTextMsg(content string, isAtAll bool, mobiles ...string) error {
	return w.SendPayload(&Payload{
		MsgType: "text",
		Text: struct {
			Content string `json:"content"`
		}{
			Content: content,
		},
		At: struct {
			AtMobiles []string `json:"atMobiles"`
			IsAtAll   bool     `json:"isAtAll"`
		}{
			AtMobiles: mobiles,
			IsAtAll:   isAtAll,
		},
	})
}

// SendLinkMsg 发送链接消息
func (w *Webhook) SendLinkMsg(title, content, picURL, msgURL string) error {
	return w.SendPayload(&Payload{
		MsgType: "link",
		Link: struct {
			Text       string `json:"text"`
			Title      string `json:"title"`
			PicURL     string `json:"picUrl"`
			MessageURL string `json:"messageUrl"`
		}{
			Text:       content,
			Title:      title,
			PicURL:     picURL,
			MessageURL: msgURL,
		},
	})
}

// SendMarkdownMsg 发送markdown消息，仅支持以下格式
/*
标题
# 一级标题
## 二级标题
### 三级标题
#### 四级标题
##### 五级标题
###### 六级标题

引用
> A man who stands for nothing will fall for anything.

文字加粗、斜体
**bold**
*italic*

链接
[this is a link](http://name.com)

图片
![](http://name.com/pic.jpg)

无序列表
- item1
- item2

有序列表
1. item1
2. item2
*/
func (w *Webhook) SendMarkdownMsg(title, content string, isAtAll bool, mobiles ...string) error {
	return w.SendPayload(&Payload{
		MsgType: "markdown",
		Markdown: struct {
			Text  string `json:"text"`
			Title string `json:"title"`
		}{
			Text:  content,
			Title: title,
		},
		At: struct {
			AtMobiles []string `json:"atMobiles"`
			IsAtAll   bool     `json:"isAtAll"`
		}{
			AtMobiles: mobiles,
			IsAtAll:   isAtAll,
		},
	})
}

// SendSingleActionCardMsg 发送整体跳转ActionCard类型消息
func (w *Webhook) SendSingleActionCardMsg(title, content, linkTitle, linkURL string, hideAvatar, btnOrientation bool) error {
	var strHideAvatar = "0"
	if hideAvatar {
		strHideAvatar = "1"
	}
	var strBtnOrientation = "0"
	if btnOrientation {
		strBtnOrientation = "1"
	}

	return w.SendPayload(&Payload{
		MsgType: "actionCard",
		ActionCard: ActionCard{
			Text:           content,
			Title:          title,
			HideAvatar:     strHideAvatar,
			BtnOrientation: strBtnOrientation,
			SingleTitle:    linkTitle,
			SingleURL:      linkURL,
		},
	})
}

// SendActionCardMsg 独立跳转ActionCard类型
func (w *Webhook) SendActionCardMsg(title, content string, linkTitles, linkURLs []string, hideAvatar, btnOrientation bool) error {
	if len(linkTitles) == 0 || len(linkURLs) == 0 {
		return errors.New("链接参数不能为空")
	}
	if len(linkTitles) != len(linkURLs) {
		return errors.New("链接数量不匹配")
	}

	var strHideAvatar = "0"
	if hideAvatar {
		strHideAvatar = "1"
	}
	var strBtnOrientation = "0"
	if btnOrientation {
		strBtnOrientation = "1"
	}

	var btns []struct {
		Title     string `json:"title"`
		ActionURL string `json:"actionURL"`
	}

	for i := 0; i < len(linkTitles); i++ {
		btns = append(btns, struct {
			Title     string `json:"title"`
			ActionURL string `json:"actionURL"`
		}{
			Title:     linkTitles[i],
			ActionURL: linkURLs[i],
		})
	}

	return w.SendPayload(&Payload{
		MsgType: "actionCard",
		ActionCard: ActionCard{
			Text:           content,
			Title:          title,
			HideAvatar:     strHideAvatar,
			BtnOrientation: strBtnOrientation,
			Buttons:        btns,
		},
	})
}

// SendLinkCardMsg 发送链接消息
func (w *Webhook) SendLinkCardMsg(msgs []LinkMsg) error {
	return w.SendPayload(&Payload{
		MsgType: "feedCard",
		FeedCard: struct {
			Links []LinkMsg `json:"links"`
		}{
			Links: msgs,
		},
	})
}
