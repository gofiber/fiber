# Security Policy

1. [Supported Versions](#versions)
2. [Reporting security problems to Fiber](#reporting)
3. [Security Point of Contact](#contact)
4. [Incident Response Process](#process)

<a name="versions"></a>
## Supported Versions

The table below shows the supported versions for Fiber which include security updates.

| Version   | Supported          |
| --------- | ------------------ |
| >= 1.12.6 | :white_check_mark: |
| < 1.12.6  | :x:                |

<a name="reporting"></a>
## Reporting security problems to Fiber

**DO NOT CREATE AN ISSUE** to report a security problem. Instead, please
send us an e-mail at `team@gofiber.io` or join our discord server via
[this invite link](https://gofiber.io/discord) and send a private message
to Fenny or any of the maintainers.

<a name="contact"></a>
## Security Point of Contact

The security point of contact is [Fenny](https://github.com/Fenny). Fenny responds
to security incident reports as fast as possible, within one business day at the
latest.

In case Fenny does not respond within a reasonable time, the secondary point
of contact are any of the [@maintainers](https://github.com/orgs/gofiber/teams/maintainers).
The maintainers are the only other persons with administrative access to Fiber's source code.

<a name="process"></a>
## Incident Response Process

In case an incident is discovered or reported, we will follow the following
process to contain, respond and remediate:

### 1. Containment

The first step is to find out the root cause, nature and scope of the incident.

- Is still ongoing? If yes, first priority is to stop it.
- Is the incident outside of our influence? If yes, first priority is to contain it.
- Find out knows about the incident and who is affected.
- Find out what data was potentially exposed.

### 2. Response

After the initial assessment and containment to our best abilities, we will
document all actions taken in a response plan.

We will create a comment in the official `#announcements` channel to inform users about
the incident and what actions we took to contain it.

### 3. Remediation

Once the incident is confirmed to be resolved, we will summarize the lessons
learned from the incident and create a list of actions we will take to prevent
it from happening again.

### Secure accounts with access

The [Fiber Organization](https://github.com/gofiber) requires 2FA authorization
for all of it's members.

### Critical Updates And Security Notices

We learn about critical software updates and security threats from these sources

1. GitHub Security Alerts
2. GitHub: https://status.github.com/ & [@githubstatus](https://twitter.com/githubstatus)
