// SPDX-License-Identifier: AGPL-3.0-or-later
// Donation links, shown under Settings → Support AnyProfile. They open in the default browser:
// GitHub or Stripe takes the payment, and AnyProfile sends nothing. An empty link hides its
// button, so a build can ship before the account behind it exists.

export type DonateLink = { id: 'github' | 'stripe'; url: string }

export const donateLinks: DonateLink[] = [
  { id: 'github', url: 'https://github.com/sponsors/D0bby999' },
  // A one-time Stripe Payment Link where the donor picks the amount ($1 to $1,000), on the
  // Stripe account of Cassau, LLC, which also runs PostClaw. Its objects carry metadata source=anyprofile.
  { id: 'stripe', url: 'https://donate.stripe.com/5kQaEW4es59c0Ey9reejK00' },
]

export const shownDonateLinks = () => donateLinks.filter((l) => l.url)
