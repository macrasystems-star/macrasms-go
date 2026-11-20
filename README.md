# MacraSMS Go SDK

Official Go SDK for [MacraSMS](https://macrasms.com), bulk SMS for Kenyan businesses: send to one number or thousands under your own sender ID, check your credit balance and follow each message's status.

## Before you start

1. Create an account at [sms.macrasms.com](https://sms.macrasms.com/signup/) and buy credits (from KES 1 by M-Pesa or card).
2. Apply for your own sender ID on the **Sender IDs** page. Once it is approved you get an **API key** for it. The API works with your own approved sender ID, not the shared Macra one.
3. Keep the API key secret: load it from an environment variable, never commit it.

## Install

```bash
go get github.com/macrasystems-star/macrasms-go
```

Go 1.18 or newer, standard library only.

## Quick start

```go
c := macrasms.NewClient(os.Getenv("MACRASMS_SENDER_ID"), os.Getenv("MACRASMS_API_KEY"))
ctx := context.Background()

res, err := c.Send(ctx, []string{"0712345678"}, "Your order is ready for collection.")
if err != nil {
    log.Fatal(err)
}
fmt.Println(res.MessageID, res.CreditsDeducted, res.BalanceRemaining)

balance, _ := c.Balance(ctx)                      // credits left
st, _ := c.MessageStatus(ctx, res.MessageID)      // st.Status
```

## Handling errors

```go
_, err := c.Send(ctx, []string{"0712345678"}, "Hello")
var apiErr *macrasms.APIError
switch {
case errors.Is(err, macrasms.ErrInsufficientBalance):
    errors.As(err, &apiErr)
    need, _ := apiErr.Int("required")
    fmt.Println("top up, need", need)
case errors.Is(err, macrasms.ErrAuthentication):
    fmt.Println("check the sender ID and API key")
case errors.Is(err, macrasms.ErrRateLimited):
    errors.As(err, &apiErr)
    wait, _ := apiErr.Int("retry_after")
    fmt.Println("retry in", wait, "seconds")
}
```

## Errors

| HTTP | Go error | Meaning |
|---|---|---|
| 400 | `ErrValidation` | A required value is missing or invalid |
| 401 | `ErrAuthentication` | The sender ID and API key do not match an approved sender ID |
| 402 | `ErrInsufficientBalance` | Not enough credits (includes the credits required and available) |
| 403 | `ErrAccountSuspended` | The account is suspended |
| 404 | `ErrNotFound` | The message does not exist or belongs to another account |
| 429 | `ErrRateLimited` | Too many requests; wait the number of seconds given |
| other | `ErrAPI` | Any other API or network problem |

## Credits and message length

One SMS holds up to 160 characters. Longer messages are sent in parts of 153 characters and each part uses one credit per recipient. Messages with emoji or characters outside the standard SMS alphabet hold 70 characters, or 67 per part. `macrasms.Segments(text)` tells you how many parts a message will use before you send it.

## Message status

`pending` means queued, then `sent`, `partial` (some recipients failed) or `failed`. Messages are usually sent within seconds.

## Links

- API documentation: https://macrasms.com/api-docs/
- Pricing and sign up: https://macrasms.com
- Support: info@macrasystems.com

## Licence

MIT, Copyright (c) 2026 Macra Systems Ltd.

## Running the tests

```bash
go test ./...
```
