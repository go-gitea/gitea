---
date: "2018-11-23:00:00+02:00"
title: "External renderers"
slug: "external-renderers"
weight: 40
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "External renderers"
    weight: 40
    identifier: "external-renderers"
---

# Custom files rendering configuration

Gitea supports custom file renderings (i.e., Jupyter notebooks, asciidoc, etc.) through external binaries, 
it is just a matter of:

* installing external binaries
* add some configuration to your `app.ini` file
* restart your Gitea instance

## Installing external binaries

In order to get file rendering through external binaries, their associated packages must be installed. 
If you're using a Docker image, your `Dockerfile` should contain something along this lines:

```
FROM gitea/gitea:{{< version >}}
[...]

COPY custom/app.ini /data/gitea/conf/app.ini
[...]

RUN apk --no-cache add asciidoctor freetype freetype-dev gcc g++ libpng python-dev py-pip python3-dev py3-pip py3-zmq
# install any other package you need for your external renderers

RUN pip3 install --upgrade pip
RUN pip3 install -U setuptools
RUN pip3 install jupyter matplotlib docutils 
# add above any other python package you may need to install
```

## `app.ini` file configuration

add one `[markup.XXXXX]` section per external renderer on your custom `app.ini`:

```
[markup.asciidoc]
ENABLED = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoctor --out-file=- -"
; Input is not a standard input but a file
IS_INPUT_FILE = false

[markup.jupyter]
ENABLED = true
FILE_EXTENSIONS = .ipynb
RENDER_COMMAND = "jupyter nbconvert --stdout --to html --template basic "
IS_INPUT_FILE = true

[markup.restructuredtext]
ENABLED = true
FILE_EXTENSIONS = .rst
RENDER_COMMAND = rst2html.py
IS_INPUT_FILE = false
```

Once your configuration changes have been made, restart Gitea to have changes take effect.
