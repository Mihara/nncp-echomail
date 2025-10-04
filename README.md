# NNCP-Echomail

TLDR: [NNCP](https://nncp.mirrors.quux.org/)-based, forum/echomail-like system, deliberately made as simple to implement as possible.

As of today, this is a prototype meant more as a conversation piece than anything to be actually used. It does, however, work to do what it set out to do as it is. A proper GUI editor might show up in the near future as time and energy permits.

See the [informal specification](SPEC.md) for details of how it is meant to function.

## Building

You need [Go](https://go.dev/) version 1.25.1 or newer. The most expedient way to build right now is [Taskfile](https://taskfile.dev/): `task build`, but you can build manually:

```bash
cd cmd/echomail-mailer
go build
cd ../../cmd/echomail-index
go build
```

## Usage

The prototype program comes in two executables:

+ `echomail-mailer`: receives messages, saving them into an on-disk storage structure, and packages messages for sending while saving them to the same on-disk storage structure (because currently, you can't receive messages you send to an area.)
+ `echomail-index`: indexes messages on disk to produce a Gemini-based group/message tree, which you can read with any Gemini browser or feed to a Gemini server.

There is currently no specialized message editor, yet, though any skilled Emacs user can probably cook something up in a couple hours.

To see how the whole system works, `task testrun` and look at the files produced in `testdata/echo`.

## Configuration

In the most obvious case, in your `nncp.hjson`:

```hjson
  areas: {
    echo: {
      id: WZ..WQ

      ...

      exec: {
        echomail: ["echomail-mailer", "receive", "-root", "/var/spool/echomail"]
      }

    }
  }
```

Or perhaps you use a shell script invoking both `echomail-mailer` and `echomail-index`.

To send a message, you create a message gemtext with the appropriate header and do something like this:

```bash
cat message.gmi | echomail-mailer send -root /var/spool/echomail | nncp-exec area:echo echomail
```

## LICENSE

This program is licensed under the terms of MIT license, for what it's worth, which is honestly not much as of yet.
