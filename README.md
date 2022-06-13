# goStonkBot

A Slack based bot to play the stonk market with others.

## Features

Uses a "real time" feed of stock data, and maintains players portfolios.

## Commands


# Quick Setup

1. Copy `.env.template` to `.env` and configure the following environment variables:
   * `REDIS_URL` - URL formatted connection string to your Redis instance.
   * `REDIS_KEY_PREFIX` - a string to a prefix for all Stonkbot related Redis keys.
   * `HTTP_SERVER_BIND` - an IP and port combination to bind the HTTP server to for Slack events.

2. Go to [Your Apps](https://api.slack.com/apps/) on Slack, and `Create New App`.
3. When prompted, select `From an app manifest`.
4. Select the Workspace you want to develop the bot in.
5. Use the contents from [manifest.yml](manifest.yml) to paste in to the manifest. Make sure to update the request_url key with the publically exposed HTTP_SERVER_BIND value.
6. Once the app has been created, install it in to the Workspace, so you can retrieve the Bot User OAuth Token under `Oauth & Permissions`, to be placed in your `.env` under `SLACK_TOKEN`
7. On the `Basic Information` page, you can get your `Signing Secret` to be placed in your `.env` under `SLACK_SIGNING_SECRET`.
8. Finally, build and run the bot via `go build .` and `./stonkbot`
