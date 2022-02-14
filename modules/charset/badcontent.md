<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>下单</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>api/Order/CreateOrder post
说明：下单
参数：
custom_app_key
session_3rd</p>
<p>remark 备注
total 总金额
freightCost 运费
addressId 客户地址ID
StockId 库存门店ID
isGiftBag 是否礼包单
ordersDetail 订单明细（以下是明细的具体内容，可以有多条）
name 商品名称
type 类型:目前有 商品 个人账户储值 个人账户消费 外卖消费券 分期贷款 这些
title 描述
sum 数量
price 销售单价
objId 相关对象ID:商品为商品ID,外卖消费券为消费券ID...
price_xz 价格修正：即团餐的差价</p>
<p>返回：
下单是否成功</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>支付</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>api/Order/PayOrder post
说明：微信支付
参数：
custom_app_key
session_3rd
orderId：销售订单ID
返回：
支付是否成功</p>
<p>api/Order/PayOrder_WJB post
说明：无尽宝支付
参数：
custom_app_key
session_3rd
orderId：销售订单ID
useCard:银行卡号
useCardPhone：预留手机
channelCode：渠道简码
period：分期期数</p>
<p>返回：
{
&#34;msg&#34;: &#34;&#34;,
&#34;code&#34;: 200,
&#34;data&#34;: {
&#34;payInfo&#34;:&#34;&#34; --支付的html页面，如果支付渠道是银联，该值是为空，后续需要获取短信验证，并使用获取到的验证码来确认支付
&#34;orderId&#34;:&#34;&#34; --这个不是销售订单ID,是支付商户订单号，后续的调用会需要
{
}</p>
<p>api/Order/RequestSmsCode_WJB post
功能：发送验证码 如果支付渠道是银联,则需要给支付方的预留手机发送验证码来确认支付，而不是同其他渠道那样通过 上面 的接口返回的html页面中完成支付
参数：
custom_app_key
orderId：支付商户订单号
返回：
调用是否成功</p>
<p>api/Order/PayConfirmed_WJB post
功能：确认支付
参数：
custom_app_key
orderId：支付商户订单号
smsCode：即调用 上面 接口后用户收到的短信验证码
返回：
调用是否成功</p>
<p>说明：
分期期数选1为不分期，非信用卡开通快捷支付都可以支付，金额最低0.1。其它数字代表分期，必须选择信用卡，金额最低100。
支付前可以选择使用 GetShopSupportPeriod 接口(后面会列出)获得推荐的渠道和期数。</p>
<p>api/Order/GetShopSupportPeriod get
功能：获取符合查询条件的银行及期数信息（目前不需要调用，原因见下面的说明）
参数：
custom_app_key
payAmt：订单（支付）金额
supplyPrice：订单成本价
payChannel：支付渠道 1-工银E支付、2-农行快E付、3-银联、5-云闪付、8-支付宝 可不填
返回：
符合查询条件的银行及期数信息列表</p>
<p>说明：
订单成本价，不能高于支付金额。等于支付金额时接口不返回可选的渠道和期数，可以不用该接口。
接口根据支付金额和成本价会推荐出可选渠道和期数，尽量使商户产生利润。(
支付金额-订单进货总价&gt;总的分期手续)</p>
<p>api/Order/GetTradeOpenCardUrl get
功能：卡片开通快捷支付
参数：
custom_app_key
accNo：卡号
phone：预留手机
返回：
进行开通操作的页面，按页面提示进行开通即可</p>
<p>api/Order/QueryTradeOpenCardResult get
说明：查询快捷支付的开通情况
参数：
custom_app_key
useCard 银联的卡号
payType 1：线上支付 2：线下支付
返回：
是否开通</p>
<p>api/Order/GetBankInfo get
说明：获取银行相关的信息
参数：
custom_app_key
channelCode 渠道简码
返回：
{
&#34;bankName&#34;:&#34;&#34;,--银行名称
&#34;channelCode&#34;:&#34;&#34;,--渠道简码
&#34;mininumAmount&#34;:&#34;&#34;,--起步金额
&#34;maxinumInstalment&#34;:&#34;&#34;--最大分期
}</p>
<p>api/Order/CancelPay post
说明：取消支付
场景是：用户下单后进入支付页面，当用户在被要求输入支付密码或验证码时取消了支付，使本次支付流程中止时需要调用本接口以强制终止支付流程，使后续再次支付或关闭订单的操作能够正常进行，否则系统会因为等待支付的完成而处于永远的等待中，当然，这个问题可以通过额外的代码干预，但目前没有这样的代码
参数：
custom_app_key
orderId 销售订单ID
返回：是否成功</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>订单信息</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>api/Order/GetOrderList get
说明：获取客户的订单列表，或配送员的接单列表
参数：
custom_app_key
session_3rd
isALL：定死填 1
isPost：1：是配送员 否则是一般的客户
payState：即支付状态,当isALL&lt;&gt;1时才有效，目前的这个状态有点乱，各类型的在线支付的这个状态不是很统一，所以不用这个
返回：
主表属性：
openid 用户小程序的openid
orderId 销售订单ID
orderNum 销售订单的单号
userId 会员用户
payState 支付状态
money 订单金额（含运费及返现修正）
freightCost 运费
goodsCost 订单金额
createDate 下单时间
orderState 订单状态
isGiftBag 是否礼包但
gradeId 礼包档次ID
afterSaleInProcessId 当前售后申请ID
existAfterSaleInProcess 是否存在处理中的申请
disposedGoodsIds 已申请售后的物品列表
postmanNickname 配送员昵称
postmanName 配送员姓名
postmanPhone 配送员手机
isJustInTime 是否即时配送
pickupTime 取货时间
finishTime 配送完成时间
orderDetails 明细列表 可能多条
goodsId 商品ID
name 内容 一般是商品名称
price 销售单价
count 数量
total 销售金额
subTitle 小标题
TuanCan 是否使用团餐价支付
images 商品图片 可能多张
Path 原图url
ThumbnailPath 缩略图url</p>
<p>api/Order/GetOrderInfo get
说明：获取指定销售订单的信息详情
参数：
custom_app_key
session_3rd
orderId 销售订单ID
返回：
openid 用户的微信小程序openid
orderId 销售订单ID
orderNum 订单的单号
userId 用户ID
payState 支付状态
money 订单金额（含运费）
freightCost 运费
goodsCost 订单金额（已扣除返现金额）
createDate 下单时间
orderState 订单状态
isGiftBag 是否礼包单
gradeId 礼包档次ID
savedMoney 节省金额
addressee 收件人
contactPhone 联系电话
province 省
city 市
area 区县
address 详细地址
longitude 经度
latitude 纬度
isDefault 是否默认地址
afterSaleInProcessId 当前售后申请ID
existAfterSaleInProcess 是否存在处理中的申请
disposedGoodsIds 已申请售后的物品列表（只返回物品ID）
orderDetails 明细
以下是明细的属性(明细可能有多条)
goodsId 物品ID
name 内容
price 销售价
count 数量
total 销售金额
subTitle 小标题
TuanCan 团餐的标记（1：使用了团餐价 0：普通价）
images 商品图片（可能有多张图）
以下是商品图片的属性
Path 原图url
ThumbnailPath 缩略图url</p>
<p>api/Order/GetGoodsTrancingInfo get
说明：扫码溯源 码是重用之前给配送员取货时提供的二维码，这里就是简单的判断一下：是配送员调用另外一个接口去绑定订单，普通客户的话就调用这个接口，返回本订单下的所有商品的溯源信息
参数：
custom_app_key
orderId：销售订单ID
返回示例：
{
&#34;orderId&#34;:&#34;1&#34;,                --销售订单ID
&#34;orderNum&#34;:&#34;MM00008888&#34;,        --销售单号
&#34;date&#34;:&#34;2021-08-10&#34;,                --下单日期(制单日期)
&#34;details&#34;:[
{
&#34;goodsId&#34;:&#34;9045&#34;,                --商品ID
&#34;goodsName&#34;:&#34;可乐&#34;,        --商品名称
&#34;trancingInfo&#34;:&#34;市场批发&#34;        --溯源信息
“images”:[
{
&#34;Path&#34;:&#34;&#34;，                        --原图url
&#34;ThumbnailPath&#34;:&#34;&#34;        --缩略图url
},
...
]
},
...
]
}</p>
<p>api/Order/GetTuanCanDifferenceInfo get
说明：获取团餐差价信息
因为目前先是比较简单的实现，这个规则是写死的，所以不需要提供参数
返回：
[
{&#34;difference&#34;:&#34;-1.5&#34;,&#34;min&#34;:&#34;5&#34;,&#34;max&#34;:&#34;9&#34;},
{&#34;difference&#34;:&#34;-2&#34;,&#34;min&#34;:&#34;10&#34;,&#34;max&#34;:&#34;19&#34;},
{&#34;difference&#34;:&#34;-2.5&#34;,&#34;min&#34;:&#34;20&#34;,&#34;max&#34;:&#34;39&#34;},
{&#34;difference&#34;:&#34;-3&#34;,&#34;min&#34;:&#34;40&#34;,&#34;max&#34;:&#34;&#34;},
]
这个数据的含义是:
5—9份（每份减1.5元）
10—19份（每份减2元）
20—39份（每份减2.5元）
40份以上（每份减3元）
自取价（每份减4元）</p>
<p>api/Order/UpdateDeductedInterest post
说明：更新已免利息，这个利息的规则是前端定死的，所以由前端在需要时计算并返回给后端
参数：
custom_app_key
orderId 销售订单ID
interestDeducted 免息金额
返回：
是否成功更新</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>售后</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>api/Order/CloseOrder post
说明：用户主动关闭订单
参数：
custom_app_key
session_3rd
orderId 销售订单ID
返回：
操作呢是否成功</p>
<p>api/Order/GetOrderInfo_SH get
说明：获取售后信息
参数：
custom_app_key
orderId 销售订单ID
返回该订单的所有申请记录（可能有多条）
applyTime 申请的发起时间
remark 备注
result 处理结果
resultTime 处理日期
details 明细
明细可能有多条，一下是每条明细所拥有的属性
goodsName 商品名称
remark 备注</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>配送</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>api/Order/PostPickupInfo post
说明：建立配送员与订单之间的绑定
参数：
custom_app_key
session_3rd
orderId 销售订单ID
返回：
操作是否成功</p>
<p>api/Order/GoodsReceived post
说明：配送员完成配送确认时需要调用
参数：
custom_app_key
session_3rd
orderId 销售订单ID
返回：
操作是否成功</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>商品信息</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>注意：以下提及的物品ID和商品ID是一个东西</p>
<p>api/Goods/GetGoodsClassify get
说明：获取商品分类信息
参数：
custom_app_key
返回的是商品的分类列表(可能多条)
每条属性：
classify 分类ID
classifyName 分类名称</p>
<p>api/Goods/GetTakeoutStoreList get
说明：获取外卖门店，因为需要根据客户的地理位置确定合适配送的门店以进而确定餐品的库存，所以这里提供出所有的门店信息以供前端匹配选择
参数：
custom_app_key
返回：
门店列表（可能多条）
每条属性：
md_id 门店ID
name 门店名称
longitude、latitude 门店的经纬度
remark 备注</p>
<p>api/Goods/GetGoodsByClassify get
说明：获取指定分类的商品
参数：
custom_app_key
classify 分类ID
ColumnId 栏目ID
StockId f8外卖需要确定指定门店商品库存的需要传此参数
返回匹配物品的列表（可能多条）
每条属性：
name 商品名称
unitPrice 最低价
price 售价
original_price 原价
price_lb 礼包价
markingPrice 划线价（目前 原价=划线价）
articleId 物品ID
isInStock 是否有库存
isStock 是否扣库（即商品在销售过程中是不是涉及库存变动）
hasRelated 是否存在关联商品
images 商品图片（可能多张）</p>
<p>api/Goods/GetGoodsList get
说明：商品列表展示
参数：
custom_app_key
ColumnId 栏目ID
StockId 库存门店ID
下面参数供分页机制使用：
page 定位的页码
num 每页的商品数
返回：
total 商品总数 是无视分页参数的商品总数
商品列表（可能多条）：
name 名称
unitPrice 最低价
price 售价
original_price 原价
price_lb 礼包价
markingPrice 划线价
articleId 物品ID
inInStock 是否有库存（只针对于需扣库存的商品）
isStock 是否扣库
hasRelated 是否存在关联商品
ColNo 栏目排序
images 商品图片（可能多张）</p>
<p>api/Goods/GetGoodsInfo get
说明：商品详情（指单个商品）
参数：
custom_app_key
goodsId 商品ID
ColumnId 栏目ID(这个参数不影响 goodsId 的商品的筛选 影响的是关联商品的过滤)
StockId 库存门店
返回：
items 商品列表（包含了关联商品）可能多条
商品属性：
name 商品名称
unitPrice 最低价
price 售价
original_price 原价
price_lb 礼包价
markingPrice 划线价
articleId 物品ID
inInStock 是否有库存
isStock 是否扣库
Inventory 库存总数
subtitle 小标题
relatedName 关联名（规格标签下使用的名称）
detailDescription 详情描述
relatedSecNo 关联排序（随便你们用不用）
weight 运费系数
images 商品图片，可能多张
规格类型列表，理论可以有多条，但目前只会是一种规格类型，并且名称就叫规格(具体去了解前端sku机制)
规格类型属性
attrID 规格类型ID =1
attr 类型名称 =规格
sort 排序 =1
attrValues 类型下的商品列表 一般多条
商品属性：
attrID =1
attrValueID 关联商品ID
attrValue 关联商品关联名</p>
<p>api/Goods/GetFreightCost get
说明：获取商品的运费
参数：
custom_app_key
weight 运费系数（在获取商品详情时会返回这个值）
返回：
如果调用成功，会在data中返回运费金额</p>
<p>api/Goods/SearchGoods get
说明：商品搜索
参数：
custom_app_key
keyword 搜索关键词（分词间需空格分隔）
ColumnId
StockId
返回：
商品列表（与获取商品的其他接口类似，不再赘述）</p>
<p>api/Goods/GetGoodsColumn get
说明：获取商品栏目
参数：
custom_app_key
返回：
栏目列表 多条栏目
栏目属性：
column 栏目ID
columnName 栏目名称</p>
<p>api/Goods/GetGiftBagGrades get
说明：获取礼包档次信息
参数：
custom_app_key
返回：
礼包档次列表 多条
礼包档次属性：
gradeId 档次ID
gradePrice 价格档次
priceFloat 允许浮动
seqNo 排序（随便你们用不用）</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>用户相关</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/</p>
<p>api/User/PostNewUser post
说明：上传或更新用户信息（注册用）
参数：
custom_app_key
session_3rd
返回：
操作是否成功</p>
<p>api/User/GetCustomerAddress get
说明：获取用户地址
参数：
custom_app_key
session_3rd
返回：
地址列表
单条地址信息的属性：
addressId 地址ID
addressee 收件人
contactPhone 联系电话
province 省
city 市
area 区县
address 详细地址
isDefault 是否默认地址
longitude 经度
latitude 纬度</p>
<p>api/User/AddCustomerAddress post
说明：添加用户地址
参数：
custom_app_key
session_3rd
receiver 收件人
phone 收件人手机、电话
province 省
city 市
area 区
address 详细地址
longitude 经度
latitude 纬度
isDefault 是否默认 1：是 0：不是
返回：
操作是否成功</p>
<p>api/User/UpdateCustomerAddress post
说明：修改用户地址
参数：
custom_app_key
session_3rd
addressId 地址ID
receiver 收件人
phone 收件人手机、电话
province 省
city 市
area 区
address 详细地址
longitude 经度
latitude 纬度
isDefault 是否默认 1：是 0：不是
返回：
操作是否成功</p>
<p>api/User/DeleteCustomerAddress post
说明：删除用户地址
参数：
custom_app_key
session_3rd
addressId 地址ID
返回：
操作是否成功</p>
<p>以下用{ }表示对象，{ }中是对象的属性,[ ]表示数组</p>
<p>api/User/ApplyAfterSaleService post
说明：申请售后
参数：
custom_app_key
session_3rd
order
{
orderId 销售订单ID
method 售后方式 （目前就3个：换货, 仅退款, 退款退货）
remark 备注
details 售后明细
[
{
goodsId 商品ID
remark 备注
}
...
]
}
返回：
售后是否提交成功，小程序的售后流程这里简单地描述一下：
用户发起申请，后端erp软件中由内部人员进行审核并进行处理，可能通过，也可能驳回，最后信息会通过小程序调用相关接口返回给用户</p>
<p>api/User/GetUserAcountInfo get
说明：获取用户账户信息
参数：
custom_app_key
session_3rd
返回：
name 姓名
phone 手机号
HY_card 会员卡号
account_balance 个人账户余额
save_money_lb 礼包节省金额
Back_money 返现金额
Earning_money 赚钱
is_postman 是否配送员
IsShowDTCode 是否显示地推码（目前可以忽略）
interestDeducted 已免利息
savedMoney 节省金额
account_water_bill 个人账户充值/消费流水
[
{
date 充值/消费日期
content 摘要
fee 涉及金额
}
...
]</p>
<p>api/User/PostUserSharingInfo post
说明：当用户点击了另一个用户分享的链接进入小程序，则调用本接口记录下这种推荐关系
参数：
custom_app_key
openid_a　推荐人
openid_b　被推荐人
url 分享链接
返回：
操作是否成功</p>
<p>api/User/GetRecommenedUserList get
说明：获取用户的邀请好友列表
参数：
custom_app_key
session_3rd
返回：
被推荐人列表
被推荐人属性：
openid 被推荐人的openid
avatarUrl 头像url
nickname 昵称
time 推荐时间</p>
<p>api/User/PostUserBankCardInfo post
说明：添加银行卡
参数：
custom_app_key
session_3rd
name 持卡人
useCard 银行卡号
useCardPhone 预留手机
channelCode 渠道简码
返回：
是否保存成功</p>
<p>api/User/DeleteUserBankCardInfo post
说明：删除银行卡
参数：
custom_app_key
session_3rd
cardId 银行卡ID
返回：
操作是否成功</p>
<p>api/User/GetUserBankCardInfo get
说明：获取银行卡信息
参数：
custom_app_key
session_3rd
返回：
银行卡信息列表 可能多条
银行卡信息属性：
cardId 银行卡ID
name 持卡人
useCard 银行卡号
useCardPhone 预留手机
channelCode 渠道简码
mininumAmount 起步金额
maxinumInstalment 最大分期</p>
<p>api/User/SendServiceMessage post
说明：调用微信服务推送功能
custom_app_key
以下参数详见微信小程序的开发文档：<a href="https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/subscribe-message/subscribeMessage.send.html" rel="nofollow">https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/subscribe-message/subscribeMessage.send.html</a>
touser
template_id
page
data
miniprogram_state
lang
返回：
发送是否成功</p>
<p>api/User/UserCheckIn post
说明：用户签到 这个接口可以随便调用，没有啥限制，想调就调，
这个签到包含了领券的操作，这个由后端处理
参数：
custom_app_key
session_3rd
返回：
操作是否成功</p>
<p>api/User/GetUserCoupons get
说明：获取消费券列表
参数：
custom_app_key
session_3rd
返回：
消费券列表 可能多条
消费券属性：
couponId 券ID
price 面值 目前固定0.2
date 领券日期</p>
<p>api/Info/GetAppInfo get
说明：获取小程序的信息（目前信息不多）
参数：
custom_app_key
返回：
Tel 就是 我的 页面中显示的那个 400 电话
price_lb_inf 礼包起步金额 这个是商城用的，外卖无视</p>
<p>/<strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong><strong>霸王餐活动</strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong></strong>******/
api/User/GenerateCoupons_BWC post
说明：形成券（对于前端来说就是邀请人点击立即使用时调用）
参数：
custom_app_key
session_3rd
invitee_ids：被邀请人的ID，多个时用 ,(逗号) 分隔 例如 1,3,9...
返回：
是否操作成功</p>
<p>api/User/GetFriendList_BWC get
说明：获取被邀请人列表
参数：
custom_app_key
session_3rd
type:1 或 2   对邀请人来说传1，这时券的面额公式为 (列表长度+1)*2；对被邀请人来说传2，这时券的面额公式为 (列表长度+2)*2
返回：
{
&#34;inviter&#34;:{ &#34;id&#34;:&#34;&#34;,&#34;nickname&#34;:&#34;&#34;,&#34;avatarUrl&#34;:&#34;&#34; },
&#34;invitees&#34;:
[
{ &#34;id&#34;:&#34;&#34;,&#34;nickname&#34;:&#34;&#34;,&#34;avatarUrl&#34;:&#34;&#34; },
...
]
}</p>
<p>api/User/InquireUserRegisterStat get
说明：查询用户是否已注册
参数：
custom_app_key
session_3rd
返回：
是否注册</p>
