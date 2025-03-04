---
title: file
type: output
status: stable
categories: ["Local"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the contents of:
     lib/output/file.go
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';


Writes messages to files on disk based on a chosen codec.

```yml
# Config fields, showing default values
output:
  label: ""
  file:
    path: ""
    codec: lines
```

Messages can be written to different files by using [interpolation functions](/docs/configuration/interpolation#bloblang-queries) in the path field. However, only one file is ever open at a given time, and therefore when the path changes the previously open file is closed.

## Batches and Multipart Messages

When writing multipart (batched) messages using the `lines` codec the last message ends with double delimiters. E.g. the messages "foo", "bar" and "baz" would be written as:

```
foo\n
bar\n
baz\n
```

Whereas a multipart message [ "foo", "bar", "baz" ] would be written as:

```
foo\n
bar\n
baz\n\n
```

This enables consumers of this output feed to reconstruct the original batches. However, if you wish to avoid this behaviour then add a [`split` processor](/docs/components/processors/split) before messages reach this output.

## Fields

### `path`

The file to write to, if the file does not yet exist it will be created.
This field supports [interpolation functions](/docs/configuration/interpolation#bloblang-queries).


Type: `string`  
Default: `""`  
Requires version 3.33.0 or newer  

```yml
# Examples

path: /tmp/data.txt

path: /tmp/${! timestamp_unix() }.txt

path: /tmp/${! json("document.id") }.json
```

### `codec`

The way in which the bytes of messages should be written out into the output data stream. It's possible to write lines using a custom delimiter with the `delim:x` codec, where x is the character sequence custom delimiter.


Type: `string`  
Default: `"lines"`  
Requires version 3.33.0 or newer  

| Option | Summary |
|---|---|
| `all-bytes` | Only applicable to file based outputs. Writes each message to a file in full, if the file already exists the old content is deleted. |
| `append` | Append each message to the output stream without any delimiter or special encoding. |
| `lines` | Append each message to the output stream followed by a line break. |
| `delim:x` | Append each message to the output stream followed by a custom delimiter. |


```yml
# Examples

codec: lines

codec: "delim:\t"

codec: delim:foobar
```


