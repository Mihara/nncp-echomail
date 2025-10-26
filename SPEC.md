# NNCP-Echomail: the Spec

This is not, as of yet, a formal spec, more of a seed for discussion and a description of how the prototype works.

The idea is not to plan for a continent-spanning network with a huge volume of messages, it is to design something that can serve the needs of a forum in a thousands strong network of nodes.

Following the general [Gemini](https://en.wikipedia.org/wiki/Gemini_(protocol)) principles, which I thought were appropriate for this use, you should be able to implement it in bash on a potato if you absolutely have to, to make adoption easier.

## Messages

1. Messages are sent to an NNCP `areas: { something: { exec: { echomail: [] } } }` -- the handle can, of course, be something else, as needed, but it will need to be standard throughout the area. Messages are packaged into an "envelope" container format together with accompanying file attachments, if any. Propagation, routing and checking for duplicates resulting from routing loops are not part of the spec at all: They are handled by NNCP area mechanics already.
2. Storage of received messages is an implementation detail. However, the idea is that messages can be simply stored on disk as a directory tree of files, once recovered from their envelopes. An index can then be built for them and read by a Gemini browser directly, enabling one to convert a Gemini browser into a mail editor. It can likewise be served by a Gemini server, enabling public display of message archives. If your implementation requires keeping them in a database, then a database it is, the spec only specifies how the links between messages and file attachments must work.
3. Messages should be verified upon reception to have originated from the NNCP ID that they have recorded inside. **In the current release of NNCP (8.13) this is impossible**, because exec handlers for areas cannot access the requisite information. A patch to enable this has been submitted and accepted, and the environment variable `NNCP_ORIGIN` will contain the ID of the originator of the packet. If the area is set `allow-unknown: true`, it will not be cryptographically verified by NNCP, in which case echomail will have no protection from impersonation, but that will be a risk the node owner will have accepted explicitly.

### Envelopes

An envelope is a trivial container for binary files that starts with a text header describing the contents:

```text
ECHO 1.0
31534 .
995345 attachment.mp3
end
```

The rules are as follows:

1. Each line of the header is terminated by `\n`, (LF).
2. The header starts with the `ECHO 1.0` format indicator. The `1.0` is the version number, allowing for future changes to envelope format.
3. Each subsequent line consists of the length of an enclosed file in bytes, as a string, followed by a space, followed by the filename of that file in UTF-8.
4. The header is terminated by `end` alone on the line.
5. Contents of the listed files follows, in the order they were listed in the header.
6. Duplicate file names are an error and must cause the entire envelope to be discarded, and so are file names containing any path separators. Spaces in the beginning and end of filenames are to be ignored.
7. It is the mailer's responsibility to test for filenames illegal on a particular system and either deal with them or reject such messages. It is a good practice to prevent users from sending envelopes containing filenames that *might* be illegal elsewhere.

The only job of the envelope is to string multiple files together, any metadata required has to be part of the message header.

The one file an envelope is required to have is `.`, which is the message itself.

### Message files

That is, the `.`.

+ A message file is a valid gemtext file.
+ Upon reception, the message's unique *msgid* is calculated.
  + The msgid is a SHA-512/256 hash of the message file, (as specified in [FIPS 180-4](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.180-4.pdf) and available in most standard libraries near you) encoded in base64, as specified in [RFC4648](https://www.rfc-editor.org/rfc/rfc4648.html) section 5, "URL and filename safe alphabet".
  + A repeat reception of a message with the same hash should never overwrite any files received previously.
  + Whether the message file remains unaltered after its hash was calculated is an implementaiton detail.
+ A message always begins with a header block, wrapped in ```` ```Echomail ```` -- that is, a gemtext preformatted text block labeled `Echomail`.
  + The header block contains `<field>: <value>` pairs, one per line, in no specific order.
  + Messages that do not begin immediately with such a block are not valid and are to be discarded.
  + Empty lines in the header make the entire header invalid.
  + Any lines longer than 1024 bytes make the entire header invalid.
  + Header fields are to be space-trimmed during header parsing.
+ Only the `Sender` and `Group` fields are required -- Sender is used to verify authenticity, if such information is available from NNCP, while Group is used to identify a specific message group within an area. Anything else is actually optional.
  + It's up to the message reader what to assume for the contents of other fields. For example, an absent `From` might be rendered using the contents of `Sender`, while an absent `Date` might be treated as a sticky, or potentially cause the reader to disregard the entire message.
  + `Group` is a string. The mailer gets to decide whether it only accepts group names it knows, whether it creates new ones upon seeing a new group name, whether it accepts some names but explicitly ignores others, and how it stores this information. Relative links between groups are *not* specified.

### Meaning of header fields

+ `Sender`: NNCP ID of the originating node, or rather, the node the message entered the area from, since gateways are in fact possible.
+ `Group`: Name of the message group. A message group is treated as its own own separate directory (located somewhere in a vacuum) for the purposes of in-message and between-message linking, forming something akin to a specific newsgroup or a forum section. The name of this directory is deliberately not specified -- the prototype implementation uses a hash, just so that you could have any symbol you like in the group name.
+ `From`: Name of the author of the message. It is up to the message reader whether to combine it with the sender or not and how to present it. A node can be used by multiple people, a From is not a Sender. The square brackets are optional and can contain a freeform address for directly contacting the author, but always as an URI - `mailto:` for email, `https://matrix.to/#/@user:matrix.org` is an option for Matrix, `misfin://`, `gemini://`, other schema for any other means of contact, etc.
+ `To`: Has no meaning other than alerting the person in the `To` that the message is directed at them, which mail readers can take advantage of. When not directed at anyone in particular, the message should be `To: All`. When the `To` field is absent, the readers should treat the message as if it was `To: All`.
+ `Date`: Message date, used by readers for sorting messages chronologically. Must be in the form `2006-01-02 15:04:05`, and is *always* assumed to be in UTC. It is the business of the reader to translate it to local time if desired.
+ `ReplyTo`: If the message is a reply to another message, its msgid goes into this field, otherwise the field is absent. This is used by the readers to build reply trees.
+ `Subj`: Message subject, primarily intended to be displayed in lists of messages.

For example:

```gemini
Sender: NMEOT4GMO5MPF27EAZWFYL5XYLVZPIVOYUVJGQ4Y7HAUCUM7HF4Q
Group: general
ReplyTo: 7Oi1O5KI5OOOwczTMmZKRmDir5kY-vm5tgc0zqr-mi4
From: Me Again [mailto:meme@example.com]
Date: 2025-09-30 18:45:00
To: Me [mailto:me@example.com]
Subj: Re:Re: Frist Psot
```

### In-message links

Beyond the header, the message is a perfectly normal gemtext file, and can contain links, including even data URLs. The resulting message should be trivial to show in a gemini browser, process into a gempub, render as HTML, etc.

Links in the message gemtext can link to attachments packed in the same envelope, that's what the envelope is for in the first place. Every message is to be rendered as if it was loaded from the url that ends with `<msgid>/index.gmi`, while any other files in the envelope are in `<msgid>/<filename>` -- i.e. a link like `=> filename.mp3 My music` points at the file `filename.mp3` in the same envelope as the message.

Links to `../<msgid>/index.gmi` and even `../<msgid>/<attachment>` must likewise be supported by readers regardless of the actual on-disk structure and point at the other messages within the same group, so that messages can explicitly link to each other.

Links to *other* groups are not specified and need not be supported: Any given mailer might be filtering other groups out.

### Other things to consider

NNCP currently does not send messages sent to the area back to originator, even optionally. This means that mailers have to be explicitly aware of this behavior, so that they can parse a message as it is sent and save it to the same storage as the messages being received.

Fortunately, you have to do something to package a message into an envelope, so you might as well give the job to the program that already has to handle the format anyway.

## Potential questions

### Why Gemtext?

While I will be the first to criticize Gemtext as a markup format, *(I love my inline italics!)* three important considerations exist:

1. It is *trivial* to parse.
2. A lot of open source software already exists to handle it, some of which can be badgered into being a full-fledged echomail editor with a little work.
3. The prospective audiences of Gemini and NNCP already intersect to a significant degree.

In my opinion, for rich text *messages,* this is a sufficient format, and anything more complex can be handled by attachments if absolutely necessary.

### Why use a hash as the message ID?

This takes care of most forms of spoofing by default: A message with even a slightly altered header would be immediately obvious.

While, e.g. [Misfin protocol](gemini://satch.xyz/misfin/) eventually decided against hash-based message IDs because it is conceivable that a perfectly identical message would be sent twice, with this echomail protocol this is extremely unlikely, as both sender ID and date are part of the message being hashed.
