# Security Policy

This policy applies to [gofiber/fiber](https://github.com/gofiber/fiber).
Apart from the supported versions it is identical to the
[gofiber organization security policy](https://github.com/gofiber/.github/blob/main/SECURITY.md).

1. [Supported versions](#supported-versions)
2. [Reporting a vulnerability](#reporting-a-vulnerability)
3. [What happens after you report](#what-happens-after-you-report)
4. [Security points of contact](#security-points-of-contact)
5. [Incident response process](#incident-response-process)
6. [Secure accounts with access](#secure-accounts-with-access)
7. [Critical updates and security notices](#critical-updates-and-security-notices)

## Supported versions

The table below shows the Fiber versions that receive security updates. Fixes
are released for the latest `v3.x` release, so upgrade to it to receive them.

| Version | Supported          |
| ------- | ------------------ |
| 3.x     | :white_check_mark: |
| < 3.0.0 | :x:                |

Middleware, storage drivers and template engines outside this repository
follow the
[gofiber organization security policy](https://github.com/gofiber/.github/blob/main/SECURITY.md).

You do not need to find out first whether your version is still supported.
Report the problem and tell us the version you use.

## Reporting a vulnerability

**Do not open a public issue, discussion or pull request for a security
problem.** Report it privately instead:

1. **GitHub private vulnerability reporting (preferred).** Open the
   **Security** tab of this repository and click **Report a vulnerability**,
   or use the form at
   [github.com/gofiber/fiber/security/advisories/new](https://github.com/gofiber/fiber/security/advisories/new)
   directly.
   Only you and the maintainers can see the report.
2. **E-mail.** Send the report to `rene@gofiber.io`.
3. **Discord.** Join our server via
   [this invite link](https://gofiber.io/discord) and send a *private* message
   to one of the maintainers. Do not post any details in public channels.

Please include as much of the following as you can:

- the affected repository, module and version (or commit)
- a description of the problem and its impact
- the steps or a minimal proof of concept to reproduce it
- a suggested fix, if you have one

## What happens after you report

- We confirm that we received the report and assess it together with you.
- We prepare the fix privately and release it for all affected supported
  versions.
- We publish a GitHub Security Advisory for the affected repository and
  request a CVE ID for it.
- We credit you as the reporter in the advisory, unless you prefer to stay
  anonymous.

Please keep the problem confidential until the advisory is published.

## Security points of contact

For security-related matters, please contact any of the maintainers through
the channels listed under
[Reporting a vulnerability](#reporting-a-vulnerability).

The maintainers are the only persons with administrative access to the source
code of the gofiber repositories and respond to security incident reports as
fast as possible, within one business day at the latest.

## Incident response process

In case an incident is discovered or reported, we will follow this process to
contain, respond and remediate:

### 1. Containment

The first step is to find out the root cause, nature and scope of the incident.

- Is the incident still ongoing? If yes, prioritize stopping it.
- Is the incident outside our influence? If yes, prioritize containment.
- Find out who knows about the incident and who is affected.
- Find out what data was potentially exposed.

### 2. Response

After the initial assessment and containment to our best abilities, we will
document all actions taken in a response plan.

We will create a comment in the official `#announcements` channel to inform
users about the incident and what actions we took to contain it.

### 3. Remediation

Once the incident is confirmed to be resolved, we will summarize the lessons
learned from the incident and create a list of actions we will take to prevent
it from happening again.

## Secure accounts with access

The [gofiber organization](https://github.com/gofiber) requires 2FA
authorization for all of its members.

## Critical updates and security notices

We learn about critical software updates and security threats from these
sources:

1. GitHub Security Alerts and Dependabot
2. GitHub: [githubstatus.com](https://www.githubstatus.com/) and
   [@githubstatus](https://twitter.com/githubstatus)
