# Raven ONE Authentication API

**NOTE -** This is currently NOT backwards compatible with the pre-2.0 versions. We'll continue to work on this new system and WILL NOT release this auth to the world without a proper migration flow.

This is the brand new system Raven will be using for authentication & authorization. It supports both regular logins, registrations, Enterprise SSO (soon) & MFA (soon). Written in **Go(lang)** to prioritize performance where it's the most critical.

## What does it allow you to do?
Raven Auth allows you to:
- Log in
- Register your account (coming soon)
- View user settings & modify them (coming soon)
- Reset your password if they need to (coming soon)
- Enterprise SSO (coming soon)
- Tokenization

This implementation also is backwards compatible with older Raven Auth versions along with older builds of websites & apps.

## Minimum requirements
- Currently supports ARM only. (x86 support coming soon)
- An active Postgres & a Redis instance
- 256MB of RAM available to be used
- A single GPU should suffice

## .env configuration
```
MailServer=""
MailUser=""
MailPassword=""
DATABASE_URL=""

# JWT configuration
JWT_PRIVATE_KEY="" #Make sure this one is long enough
JWT_PUBLIC_KEY=""
JWT_ISSUER="https://auth.raven.co.com"

# Redis Configuration
RedisHost=""
RedisPort=""
RedisPassword=""
```