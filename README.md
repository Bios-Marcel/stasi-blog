# stasi-blog

A small generator for **sta**tic and **si**mple **blog**s.

## Overview

- [Building](#building)
- [Usage](#Usage)
- [Example](#example)
- [Features](#features)

## Building

First, you need to download Golang.

Next, you can execute the following:

```sh
go run . --input="example"
```

This will compile all files needed for the page.

To test the output, run:

```sh
go run demo/server.go
```

Then open [localhost:8080](http://localhost:8080).

**DO NOT USE THE DEMO SERVER IN PRODUCTION!**

## Usage

There's an input and an output folder. Both of these can be specified via
respective parameters.

As for the expect folder structure, the following is expected:

```plain
input
|--media             <-- Images, Videos and such
|--pages             <-- Optional static pages
|  |--about.html     <-- Example page
|--articles          <-- Contains blog posts
|  |--post-one.html  <-- Example post
|--config.json       <-- Basic page information
|--favicon.ico       <-- Icon to show in browser; To disable, set "UseFavicon" to false
```

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
   "AddOptionalMetaData": true,
   "DateFormat": "2 January 2006",
   "UseFavicon": true,
}
```

The date is in RFC3339 format and the following properties are optional:

- `BasePath` (Needed if files aren't served at domain-root)
- `Author` (Used for metadata/RSS)
- `URL` (Used for metadata/RSS)
- `Description` (Used for metadata/RSS)
- `Email` (Used for RSS)
- `CreationDate` (Used for metadata/RSS)
- `UtterancesRepo` (Needed for comments)
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

## Example

An example can be found in the `example` folder at the root of the repository.

## Features

- Article overview
- RSS Feed
- Mobile friendly
- Automatic Darkmode / Lightmode
- Custom Pages (Example would be an About page)
- Fast to load even with a slow (less than 64kbit/s) internet connection

### Desktop-only features

* Optional `Tags` sidebar + Tag filtering

### Feature only available with JS

* Comments via utteranc.es (via GitHub issues)

### Future

I might add the option to write posts with Markdown. My main goal was the
ability to write posts with HTML, but as that basically comes for
free, Markdown will be an add-on.
