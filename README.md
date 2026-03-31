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

**3X-UI** — advanced, open-source web-based control panel designed for managing Xray-core server. It offers a user-friendly interface for configuring and monitoring various VPN and proxy protocols.

> [!IMPORTANT]
> This project is only for personal usage, please do not use it for illegal purposes, and please do not use it in a production environment.

As an enhanced fork of the original X-UI project, 3X-UI provides improved stability, broader protocol support, and additional features.

## Quick Start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

For full documentation, please visit the [project Wiki](https://github.com/MHSanaei/3x-ui/wiki).

## Stripe Payment Integration

3X-UI supports optional Stripe billing so that each VPN client can be linked to a recurring Stripe subscription. When a client is added through the panel, a Stripe Customer and Subscription are created automatically.

### How it works

```
Admin adds VPN client
       │
       ▼
Stripe Customer created  ──►  Stripe Subscription created
                                        │
                              ┌─────────┴─────────┐
                         invoice.paid        invoice.payment_failed
                              │                    │
                         Client enabled       Client disabled
```

Webhook events handled automatically:

| Stripe event | Action |
|---|---|
| `invoice.paid` | Enable the VPN client |
| `invoice.payment_failed` | Disable the VPN client |
| `customer.subscription.deleted` | Disable the VPN client, mark canceled |
| `customer.subscription.updated` | Sync subscription status |

### Setup

#### 1. Create a Stripe Product & Price

In your [Stripe Dashboard](https://dashboard.stripe.com/products):
1. Create a **Product** (e.g. "VPN Monthly Plan")
2. Add a **Price** with a recurring interval (e.g. $9.99/month)
3. Copy the **Price ID** (starts with `price_...`)

#### 2. Configure the panel settings

In the 3X-UI panel go to **Settings → Stripe** and fill in:

| Setting | Description |
|---|---|
| **Enable Stripe** | Toggle the integration on/off |
| **Secret Key** | Your Stripe secret key (`sk_live_...` or `sk_test_...`) |
| **Webhook Secret** | Signing secret from the webhook endpoint (`whsec_...`) |
| **Price ID** | The recurring Price ID created above (`price_...`) |

#### 3. Register the webhook endpoint in Stripe

In **Stripe Dashboard → Developers → Webhooks** add an endpoint:

```
https://your-panel-domain.com/stripe/webhook
```

Subscribe to these events:
- `invoice.paid`
- `invoice.payment_failed`
- `customer.subscription.updated`
- `customer.subscription.deleted`

Copy the **Signing secret** (`whsec_...`) and paste it into the panel's **Webhook Secret** field.

#### 4. Test with Stripe CLI (optional)

```bash
stripe listen --forward-to https://your-panel-domain.com/stripe/webhook
stripe trigger invoice.paid
```

### Notes

- Stripe integration is **disabled by default**. Existing clients are unaffected until you enable it.
- The Stripe Customer / Subscription is created **asynchronously** after the client is saved — panel responsiveness is not impacted.
- If a client has no email address set, no Stripe record is created for that client.
- A client can be re-enabled manually from the panel regardless of Stripe status.

## A Special Thanks to

- [alireza0](https://github.com/alireza0/)

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Support project

**If this project is helpful to you, you may wish to give it a**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
