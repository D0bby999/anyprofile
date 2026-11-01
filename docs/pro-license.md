# Pro licence v2

AnyProfile Pro is sold as a subscription through Stripe (Cassau, LLC), but the
app does not trust a bearer key for daily use. Activation exchanges the key for
a signed **JWS Ed25519 certificate** bound to a device key. The certificate is
verified offline; the server is only asked again when the snapshot is due.

## What the app stores

- The Pro key, the certificate and the 32-byte device seed live in the
  encrypted vault. The vault key is in the macOS Keychain (Credential Manager
  on Windows).
- The device public key is sent to the server; the private seed never leaves
  the computer.
- `pro_state` remains a cache during the legacy migration. A verified
  certificate is the source of truth.

## Licence kinds

| kind | Use | Certificate TTL | Offline grace | Updates |
|---|---|---|---|---|
| subscription | Stripe month/year | 7 days | paid-through + 7 days | paid-through |
| gift | promo or paid gift | 90 days | licence end + 30 days | licence end |
| lifetime | owner/admin | 5 years | certificate + 90 days | 12 months by default |
| test | owner/admin | 1 day | 1 day | 1 day |

`updates_through = 0` means no update limit (Founder lifetime). An installed
pack/runtime is kept after the update window; only new downloads stop.

## Grant yourself Pro (no Stripe)

```bash
PRO_ADMIN_URL=https://pro.anytools.world \
PRO_ADMIN_KEY='…' \
./pro/tools/admin.sh grant --email me@example.com --kind lifetime --updates-days 365
```

The command prints a key once. Enter it in **Settings → Pro → Pro key**, or
give it to the app on another computer. The server stores only its hash.

For a short test:

```bash
PRO_ADMIN_KEY='…' ./pro/tools/admin.sh grant --kind test --days 7
```

## Gift a licence

```bash
PRO_ADMIN_KEY='…' ./pro/tools/admin.sh gift --kind promo --days 30 --max-devices 1
```

The code is printed once; the app has a **Gift code** field. A customer can buy
a gift from **Settings → Pro → Buy a gift**: Stripe uses `mode=payment` and the
`pro_gift` lookup key, then the code is emailed. `--kind paid` marks an
admin-created paid code (no expiry; the licence starts when redeemed). If the
recipient already has a licence, redeeming extends it instead of creating a
second key.

## Customer device management

`/vi/manage` and `/en/manage` have a **Devices** form: the customer enters
their email and gets a 20-minute HMAC link to `/manage/devices`. The page lists
their licences and devices, and can remove one device or all of them without a
password. The link is stateless; the page checks that every licence belongs to
the email in the token.

## Revoke and free a device

```bash
PRO_ADMIN_KEY='…' ./pro/tools/admin.sh list --email buyer@example.com
PRO_ADMIN_KEY='…' ./pro/tools/admin.sh reset --license lic_…
PRO_ADMIN_KEY='…' ./pro/tools/admin.sh revoke --license lic_…
```

Every admin action is written to `audit_events`. Production should put
`/admin/*` behind Cloudflare Access and set `ADMIN_LICENSE_KEY_HASH` to the
SHA-256 of the scoped key; the Worker also accepts the older `ADMIN_KEY` for
the pack CI during the transition.

## Stripe

The existing subscription checkout and webhooks create `source=stripe`,
`kind=subscription` licences. `sync` keeps `paid_through` in line with Stripe
and preserves a gift extension in `bonus_through`. Refunds and disputes revoke
the licence. The old `/v1/pack` key path is still served for one release.

## Keys and rotation

- `PRO_LICENSE_PRIVATE_KEY` is an Ed25519 seed (base64, 32 bytes) known only to
  the Worker secret store.
- `PRO_LICENSE_PUBLIC_KEY` is embedded in the app; `PRO_LICENSE_KID` travels in
  the JWS header.
- The app knows the current and previous key ids. Publish an app that knows the
  new public key **before** signing with it; keep the old key until its
  certificates expire.
- The local dev key in `pro/server/.dev.vars` is not a production key. Rotate
  it before selling.

## Tests

```bash
cd pro/server && npm test
cd frontend && npm test
GOCACHE="$TMPDIR/gocache" go test ./internal/pro ./internal/service
```

`TestProLive` / `TestProGiftLive` are opt-in and use a running Worker plus a
key/code from `admin.sh`:

```bash
ANYPROFILE_PRO_TEST_URL=http://127.0.0.1:8787 \
ANYPROFILE_PRO_TEST_KEY=APRO-… \
go test ./internal/service -run TestProLive -v
```

## Stripe gift price

The paid-gift checkout looks up a one-time price with lookup key `pro_gift`.
Live prices are **USD-only**:

- `pro_month`: USD 8.00
- `pro_year`: USD 64.00
- `pro_gift`: USD 64.00, one-time

The old VND price objects were deactivated and renamed to
`pro_*_legacy_vnd`; Stripe does not allow deleting a currency option from a
price object. The test account still has an older VND-enabled gift price
(`price_1UWaWxCZvzBarqx9ehiKSsRQ`) for local tests; do not copy it to live.

`GET /v1/prices` exposes monthly/yearly subscription prices; the gift checkout
uses the separate `pro_gift` lookup.

## Live Stripe IDs (created 2026-10-31)

- Product: `prod_VXgnRLoAV72gLn` — AnyProfile Pro
- Monthly: `price_1UWbUNCZvzBarqx92pTZM8qp` — USD 8.00, `pro_month`
- Yearly: `price_1UWbUNCZvzBarqx9joxI4cMo` — USD 64.00, `pro_year`
- Gift: `price_1UWbUOCZvzBarqx9BgSw5UWE` — USD 64.00 one-time, `pro_gift`
- Existing live Customer Portal: `bpc_1TtSwFCZvzBarqx98TGoXuCT`
  (cancel, payment method, invoices; plan switching is disabled)
- Live webhook: `we_1UWeZBCZvzBarqx9K73xAT8G` — `https://pro.anytools.world/stripe/webhook`
- Live secrets `STRIPE_SECRET_KEY` / `STRIPE_WEBHOOK_SECRET` were set on the
  `anyprofile-pro` Worker. The Worker code is not deployed yet.

The old VND prices were deactivated and renamed `pro_*_legacy_vnd`; Stripe
cannot remove a currency option from an existing price. Checkout now always
uses USD, so a Vietnamese buyer is charged in USD.

The live webhook endpoint still needs to be created with a key that has
`webhook_write`; the current restricted key lacks it.

## Production deployment (2026-10-31)

- Worker: `anyprofile-pro`, custom domain `https://pro.anytools.world`
- D1: `anyprofile-pro` (`575cfd2e-fec6-4ec6-8552-4bc330759426`)
- R2: `anyprofile-pro-packs`
- Live webhook: `we_1UWeZBCZvzBarqx9K73xAT8G`
- Production licence `kid`: `lic-2026-10`
- Production secrets set: Stripe, licence private key, admin hash, download HMAC,
  key-encryption key, manage HMAC, pack admin.
- `/v1/prices` verified via Cloudflare: USD 8 / USD 64, no VND.
- Admin grant + activate + JWS verification against production passed via a
  resolved-IP smoke test.

If `pro.anytools.world` does not resolve on this Mac while `dig` does, flush
the macOS DNS cache (`sudo dscacheutil -flushcache; sudo killall -HUP mDNSResponder`)
or wait for the resolver cache. The DNS record itself exists.

## Email

### Current production mail (2026-11-01)

The Worker sends through Stalwart at `mail.anytools.world:465` using SMTP AUTH.
Variables: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_FROM`; secret:
`SMTP_PASS`. The bootstrap admin credentials are in `pro/server/.mail-admin`
(mode 600, never commit). A test gift email was sent to the owner address.


The Worker now has a `send_email` binding (`EMAIL`) and uses **Cloudflare Email
Sending** first, then Resend if `RESEND_API_KEY` is set, then a log line.

To enable Cloudflare Email Sending for `anytools.world`:

1. Cloudflare dashboard → **Email** → **Email Sending** (open beta).
2. Enable the domain `anytools.world` / sender `pro@anytools.world`.
3. Add the DNS records the dashboard shows.
4. Send a test to your own address.

The current Wrangler OAuth token cannot enable it (`Unauthorized [code: 2036]`);
use the dashboard or an API token with **Email Sending: Edit**.

Resend is not required if Cloudflare Email Sending is enabled. If you prefer
Resend, create an API key and set `RESEND_API_KEY`; the code uses it as fallback.

## Production checklist

1. `wrangler login` (or set `CLOUDFLARE_API_TOKEN`) — the current CLI session
   is expired.
2. Generate a real licence key offline and keep the private value in
   1Password:

   ```bash
   ./pro/tools/make-license-key.sh
   ```

3. Generate a scoped admin key:

   ```bash
   ./pro/tools/make-admin-key.sh
   ```

4. Put the values in the Worker secrets:

   ```bash
   cd pro/server
   npx wrangler secret put PRO_LICENSE_PRIVATE_KEY
   npx wrangler secret put ADMIN_LICENSE_KEY_HASH
   npx wrangler secret put RESEND_API_KEY
   npx wrangler secret put MANAGE_HMAC_KEY   # optional; falls back to DOWNLOAD_HMAC_KEY
   ```

   Keep `PRO_ADMIN_KEY` for `pro/tools/admin.sh` in 1Password only. Do not put
   it in the app or in `.dev.vars` on a shared machine.

5. Put the matching public key and `PRO_LICENSE_KID` in `wrangler.jsonc`, build
   a new app with that public key, publish the app **before** signing with the
   new kid, then deploy.
6. In Cloudflare Zero Trust → Access → Applications, create a self-hosted app
   for `pro.anytools.world/admin` with a service token (or email OTP). Put the
   service token id/secret in `CF_ACCESS_CLIENT_ID` / `CF_ACCESS_CLIENT_SECRET`
   for `admin.sh`.
7. Deploy staging and run the remote smoke test, then production. Local
   staging verification was run with `wrangler dev --env staging --local`.
8. Rebuild the Mac app with **`make build`**, not plain `wails build`: the
   Makefile copies `fury-config` into `Contents/Resources/` and re-signs the
   bundle. A plain Wails build produces
   `fury-config helper is missing from the app bundle`.
