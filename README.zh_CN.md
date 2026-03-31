[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](https://github.com/MHSanaei/3x-ui/actions)
[![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](https://github.com/MHSanaei/3x-ui/releases/latest)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Go Reference](https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v2.svg)](https://pkg.go.dev/github.com/mhsanaei/3x-ui/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/mhsanaei/3x-ui/v2)](https://goreportcard.com/report/github.com/mhsanaei/3x-ui/v2)

**3X-UI** — 一个基于网页的高级开源控制面板，专为管理 Xray-core 服务器而设计。它提供了用户友好的界面，用于配置和监控各种 VPN 和代理协议。

> [!IMPORTANT]
> 本项目仅用于个人使用和通信，请勿将其用于非法目的，请勿在生产环境中使用。

作为原始 X-UI 项目的增强版本，3X-UI 提供了更好的稳定性、更广泛的协议支持和额外的功能。

## 快速开始

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

完整文档请参阅 [项目Wiki](https://github.com/MHSanaei/3x-ui/wiki)。

## Stripe 付款集成

3X-UI 支持可选的 Stripe 计费功能，可以将每个 VPN 客户端与一个 Stripe 订阅绑定。当管理员通过面板添加客户端时，系统会自动在 Stripe 中创建对应的 Customer（客户）和 Subscription（订阅）。

### 工作原理

```
管理员添加 VPN 客户端
       │
       ▼
创建 Stripe Customer  ──►  创建 Stripe Subscription
                                     │
                           ┌─────────┴─────────┐
                      invoice.paid      invoice.payment_failed
                           │                   │
                      启用 VPN 客户端      禁用 VPN 客户端
```

自动处理的 Webhook 事件：

| Stripe 事件 | 操作 |
|---|---|
| `invoice.paid` | 启用 VPN 客户端 |
| `invoice.payment_failed` | 禁用 VPN 客户端 |
| `customer.subscription.deleted` | 禁用客户端，标记为已取消 |
| `customer.subscription.updated` | 同步订阅状态 |

### 配置步骤

#### 1. 在 Stripe 中创建产品和价格

在 [Stripe 控制台](https://dashboard.stripe.com/products) 中：
1. 创建一个**产品**（例如"VPN 月度套餐"）
2. 添加一个**定期计费价格**（例如 ¥68/月）
3. 复制**价格 ID**（以 `price_` 开头）

#### 2. 在面板中配置 Stripe 设置

进入 3X-UI 面板的 **设置 → Stripe**，填写以下字段：

| 设置项 | 说明 |
|---|---|
| **启用 Stripe** | 开启或关闭该集成 |
| **Secret Key（密钥）** | 你的 Stripe 密钥（`sk_live_...` 或 `sk_test_...`） |
| **Webhook Secret** | Webhook 端点的签名密钥（`whsec_...`） |
| **Price ID（价格 ID）** | 上面创建的定期价格 ID（`price_...`） |

#### 3. 在 Stripe 中注册 Webhook 端点

在 **Stripe 控制台 → 开发者 → Webhooks** 中添加端点：

```
https://你的面板域名/stripe/webhook
```

订阅以下事件：
- `invoice.paid`
- `invoice.payment_failed`
- `customer.subscription.updated`
- `customer.subscription.deleted`

复制**签名密钥**（`whsec_...`）并填入面板的 **Webhook Secret** 字段。

#### 4. 使用 Stripe CLI 测试（可选）

```bash
stripe listen --forward-to https://你的面板域名/stripe/webhook
stripe trigger invoice.paid
```

### 注意事项

- Stripe 集成默认**关闭**，启用前现有客户端不受影响。
- Stripe Customer 和 Subscription 在客户端保存后**异步创建**，不影响面板响应速度。
- 如果客户端没有设置邮箱地址，则不会为其创建 Stripe 记录。
- 无论 Stripe 订阅状态如何，管理员都可以在面板中手动启用或禁用客户端。

## 特别感谢

- [alireza0](https://github.com/alireza0/)

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (许可证: **GPL-3.0**): _增强的 v2ray/xray 和 v2ray/xray-clients 路由规则，内置伊朗域名，专注于安全性和广告拦截。_
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (许可证: **GPL-3.0**): _此仓库包含基于俄罗斯被阻止域名和地址数据自动更新的 V2Ray 路由规则。_

## 支持项目

**如果这个项目对您有帮助，您可以给它一个**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## 随时间变化的星标数

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui) 
