# stasi-blog

A small generator for **sta**tic and **si**mple **blog**s.

## Overview

- [Building](#building)
- [Usage](#Usage)
- [Example](#example)
- [Features](#features)

## Building

If you don't want to use one of the release versions from the release section
or there's no binary for your platform, you can easily build the project
yourself.

All you need is a terminal and [Golang 1.16 or later](https://golang.org/dl/).

Second, you need to download the source code. You can either do this via the
GitHub webpage or use `git`, which you can get [here](https://git-scm.com/downloads)
or from your distributions package manager if you are on Linux.

To produce a self-contained binary, open your terminal, navigate to the
directory where you downloaded the source code to and run:

```sh
go build .
```

This will produce a binary called `stasi-blog` or `stasi-blog.exe` if you are
on Windows.

To test whether everything works, you can compile and run the example:

```sh
./stasi-blog build ./example --output="example-output"
./stasi-blog serve ./example-output
```

Then open [localhost:8080](http://localhost:8080) in your browser.

**DO NOT USE THE DEMO SERVER IN PRODUCTION!**

## Usage

There's an input and an output folder. Both of these can be specified via
the respective parameters `input` and `output`.

As for the input folder structure, the following is expected:

```plain
input
|--media             <-- Images, Videos and such
|--pages             <-- Optional static pages
|  |--about.html     <-- Example page
|--articles          <-- Contains blog posts
|  |--post-one.html  <-- Example post
|--config.json       <-- Basic page information
|--favicon.ico/png   <-- Icon to show in browser; To disable, set "UseFavicon" to false
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

The following properties are optional:

- `BasePath` (Needed if files aren't served at domain-root)
- `Author` (Used for metadata/RSS)
- `URL` (Used for metadata/RSS)
- `Description` (Used for metadata/RSS; RFC3339 format)
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

To view all available parameters, run:

```shell
./stasi-blog --help
```

## Example

An example can be found in the `example` folder at the root of the repository.

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
