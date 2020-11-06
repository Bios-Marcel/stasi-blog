# stasi-blog

A small generator for **sta**tic and **si**mple **blog**s.

## Features

* Article overview
* RSS Feed
* Mobile friendly
* Automatic Darkmode / Lightmode
* Custom Pages (Example would be an About page)
* Fast to load even with a shitty internet connection

### Desktop-only features

* Optional `Tags` sidebar + Tag filtering

### Feature only available with JS

* Comments via utteranc.es (via GitHub issues)

## Non-Goals

* Easy to write articles (It's HTML)
* Easy to customize

## Building

First, you need to download Golang and then execute the following:

```sh
go run .
```

This will compile all files needed for the page.

To test, run:

```sh
go run demo/server.go
```

Then open [localhost:8080](http://localhost:8080).

## How to use it

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
```

An example for the `config.json`:

```json
{
   "SiteName":"My Blog",
   "Author":"Firstname Lastname",
   "URL":"https://github-handle.github.io",
   "Description":"something descriptive",
   "Email":"mail@provider.com",
   "CreationDate":"2018-05-27T00:00:00+00:00",
   "UtterancesRepo": "github-handle/github-handle.github.io"
}
```

The date is in RFC3339 format and the following properties are optional:

* `Author` (Used for metadata/RSS)
* `URL` (Used for metadata/RSS)
* `Description` (Used for metadata/RSS)
* `Email` (Used for RSS)
* `CreationDate` (Used for metadata/RSS)
* `UtterancesRepo` (Needed for comments)

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

## Fonts used

Thanks to Kev Quirk for his post on local fonts:

https://kevq.uk/how-local-fonts-can-save-the-environment/

I just copied it over and trust his judgement!
Personally, I don't care about the exact font used as long as it looks okay.
I also value sites that load fast and obviously, little amount of things to
download will help.
