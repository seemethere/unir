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

# Installation!

To start using `unir` first add a configuration file called `.unir.yml` to the base directory of
your master branch.

### Example `.unir.yml`:

```yaml
# The list of people who can trigger an auto-merge with approvals
whitelist:
    - seemethere
    - botmethere

# At least 2 approvals are needed for auto-merging
approvals_needed: 2

# If one review requests changes, unir will not merge automatically
consensus_needed: true
```

### Github installation:

Installation for your repo is simple just use the following URL to add `unir` to your project!

https://github.com/apps/unir
