![robot dancing](https://media.giphy.com/media/tczJoRU7XwBS8/giphy.gif)

# unir

> unir (verb): to combine, to mix together

Utilizing the built in Github review system `unir` makes it so that
code merge is handled autonomously! All you need to do is `Approve`
changes on the Github UI and `unir` will take care of the rest!

# Building

```
make build
```

# Usage as a docker image!

Images are up on Docker Hub!

Usage is as follows:

```
docker run -d -e UNIR_WEBHOOK_SECRET -e UNIR_CLIENT_TOKEN -p 8080:8080 seemethere/unir:latest
```
