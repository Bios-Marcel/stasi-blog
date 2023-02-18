# stasi-blog

A small generator for **sta**tic and **si**mple **blog**s.

## Overview

- [stasi-blog](#stasi-blog)
  - [Overview](#overview)
  - [Installation](#installation)
  - [Usage](#usage)
  - [Documentation](#documentation)
  - [Features](#features)
    - [Desktop-only features](#desktop-only-features)
    - [Feature only available with JS](#feature-only-available-with-js)
    - [Future](#future)


## Installation

If you don't want to use one of the release versions from the release section
or there's no binary for your platform, you can easily build the project
yourself.

All you need is a terminal and [Golang 1.18 or later](https://golang.org/dl/).

Run:

```sh
go install github.com/Bios-Marcel/stasi-blog@latest
```

In order to update, run the same command again.

The produced binary will be installed at `$GOPATH/bin`. If your `GOPATH` isn't
set, the default location should be `~/go`. On my machine, this would be
`/home/marcel/go/bin`. For more information consult the
[official documentation](https://golang.org/doc/gopath_code).

In order to be able to run the tool via your terminal, you have to put
`~/go/bin` onto your `PATH` variable.

## Usage

In order to initialise a fresh blog (derived from the example), run:

```shell
stasi-blog init TARGET_FOLDER
```

Replace `TARGET_FOLDER` with whatever you want the target directory
to be. There's a basic `README.md` in the generated directory.

Running the example will explain how things work, as the example is
self-documenting.

TODO Review rest / Update docs.md

An example for the `config.json`:

```json
{
   "BasePath": "",
   "SiteName":"My Blog",
   "Author":"Firstname Lastname",
   "URL":"https://github-handle.github.io",
   "Description":"something descriptive",
   "Email":"mail@provider.com",
   "CreationDate":"2018-05-27T00:00:00+00:00",
   "UtterancesRepo": "github-handle/github-handle.github.io",
   "MaxIndexEntries": 10,
   "AddOptionalMetaData": true,
   "DateFormat": "2 January 2006",
   "UseFavicon": true,
}
```

The following properties are optional:

- `BasePath` (Needed if files aren't served at domain-root)
- `Author` (Used for metadata/RSS)
- `URL` (Used for metadata/RSS)
- `Description` (Used for metadata/RSS; RFC3339 format)
- `Email` (Used for RSS)
- `CreationDate` (Used for metadata/RSS)
- `UtterancesRepo` (Needed for comments)
- `MaxIndexEntries` (Decides how many posts are shown per page (Default 10))
- `AddOptionalMetaData` (Add metadata such as tags, description, author and so on)
- `DateFormat` (Needed for human readable dates later on)
  > [The format requires specific numbers](https://golang.org/pkg/time/#pkg-constants), it's weird.
- `UseFavicon` (Decides whether to look for a favicon; true by default)

The content of the `pages` folder will be added as stand-alone pages. Those
will show up in the header of the page and do not offer a comment-section.
Nor will they be part of the RSS feed.

The `articles` folder is where you blog posts go. Each post will be added to
the RSS feed upon page generation. Articles are written in plain HTML and you
can reference any media file with `/media/FILE.EXTENSION`. How you structure
the content of `/media` is up to you.

After compiling all your input, the data gets written into the folder
specified as output, which is `./output` by default. All data that was
previously written to the output folder will be deleted. Manually created
files however will be kept.

To view all available parameters, run:

```shell
./stasi-blog --help
```

## Documentation

More documentation can be found in [DOCS.md](/DOCS.md).

## Features

- Article overview
- RSS Feed
- Mobile friendly
- Automatic Darkmode / Lightmode
- Custom Pages (Example would be an About page)
- Fast to load even with a slow (less than 64kbit/s) internet connection

### Desktop-only features

- Optional `Tags` sidebar + Tag filtering

### Feature only available with JS

- Comments via utteranc.es (via GitHub issues)

### Future

I might add the option to write posts with Markdown. My main goal was the
ability to write posts with HTML, but as that basically comes for
free, Markdown will be an add-on.
