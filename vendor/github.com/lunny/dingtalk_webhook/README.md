# 非官方 Dingtalk webhook Golang SDK

## 此工程仅封装了 Dingtalk 的 webhook 部分的请求

## 使用

首先在dingtalk中创建一个机器人，将accessToken拷贝出来，然后执行下面方法即可

```Go
webhook := dingtalk.Webhook(accessToken)
webhook.SendTextMsg("这是一个没有AT的文本消息", false)
```

## License

This project is licensed under the MIT License.
See the [LICENSE](https://github.com/lunny/webhook_dingtalk/blob/master/LICENSE) file
for the full license text.