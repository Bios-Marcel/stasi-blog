package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"html/template"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Bios-Marcel/feeds"
	"github.com/goccy/go-yaml"
	"github.com/otiai10/copy"
	"golang.org/x/net/html"
)

//go:embed skeletons/*
var skeletonFS embed.FS

type ArticleHeaders struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Date        string `yaml:"date"`
	dateParsed  time.Time
	Tags        []string `yaml:"tags"`
	// Draft will prevent inclusion of the given page in a non-draft build.
	Draft bool `yaml:"draft"`
	// Hidden will not show any links to the given page. This works for both
	// custom pages and articles.
	Hidden bool `yaml:"hidden"`

	Author      string `yaml:"author"`
	AuthorEmail string `yaml:"author-email"`

	PodcastAudio string `yaml:"podcast-audio"`
}

func (headers *ArticleHeaders) Parse() error {
	var errs []error

	if headers.Date != "" {
		dateParsed, err := time.Parse("2006-01-02", headers.Date)
		if err != nil {
			errs = append(errs, err)
		}
		headers.dateParsed = dateParsed
	}

	return errors.Join(errs...)
}

// Builder contains anything that is shared between builds. Multiple builds
// occur during live mode, where changes are constantly applied to the running
// server.
type Builder struct {
	templates *template.Template
}

func NewBuilder() (*Builder, error) {
	builder := &Builder{}

	var err error
	builder.templates, err = template.New("").
		Funcs(template.FuncMap{
			// Both sub and add are used for the paging numbers
			"sub": func(a, b int) int {
				return a - b
			},
			"add": func(a, b int) int {
				return a + b
			},
		}).
		ParseFS(skeletonFS, "skeletons/*.html")
	if err != nil {
		return nil, fmt.Errorf("couldn't parse HTML templates: %w", err)
	}

	return builder, nil
}

func (builder *Builder) Build(
	sourceDir, outputDir, configPath string,
	minifyOutput, includeDrafts bool,
) error {
	if err := cleanup(outputDir); err != nil {
		return fmt.Errorf("error performing cleanup: %w", err)
	}

	err := createDirectories(
		filepath.Join(outputDir, "media"),
		filepath.Join(outputDir, "articles"),
		filepath.Join(outputDir, "pages"),
	)
	if err != nil {
		return fmt.Errorf("error preparing target folder structure: %w", err)
	}

	blogConfig := blogConfig{
		DateFormat:      "2 January 2006",
		MaxIndexEntries: 10,
	}
	if configPath == "" {
		configPath = filepath.Join(sourceDir, "config.json")
	}
	configFile, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("error loading config '%s': %w", configPath, err)
	}

	if err := json.NewDecoder(configFile).Decode(&blogConfig); err != nil {
		log.Fatalf("Error decoding config: %s\n", err)
	}
	if blogConfig.BasePath != "" {
		// Making sure there's not too many or too little slashes ;)
		blogConfig.BasePath = "/" + strings.Trim(blogConfig.BasePath, `/\`)
	}

	blogConfig.Favicon, err = copyFavicon(sourceDir, outputDir)
	if err != nil {
		return fmt.Errorf("error copying favicon: %w", err)
	}

	if *verbose {
		if blogConfig.Favicon == "" {
			log.Println("Warning: Neither 'favicon.ico' nor 'favicon.png' were found, is this intentional?")
		} else {
			log.Printf("Using favicon '%s'.\n", blogConfig.Favicon)
		}
	}

	if *verbose {
		log.Printf("Indexing and writing custom pages ...\n")
	}

	customPageFiles, err := os.ReadDir(filepath.Join(sourceDir, "pages"))
	if err != nil {
		return fmt.Errorf("couldn't handle pages directory: %w", err)
	}

	// We collect these to display them on the page header.
	customPages := make([]*customPageEntry, len(customPageFiles))

	for index, customPage := range customPageFiles {
		customPageSkeletonClone, err := builder.templates.Lookup("page").Clone()
		if err != nil {
			return fmt.Errorf("couldn't clone 'page' template: %w", err)
		}

		sourcePath := filepath.Join(sourceDir, "pages", customPage.Name())
		headers, rawCustomPage, err := parsePage(sourcePath)
		if err != nil {
			return fmt.Errorf("error parsing page '%s': %w", customPage.Name(), err)
		}

		if !includeDrafts && headers.Draft {
			if *verbose {
				fmt.Printf("Skipping page draft '%s'\n", customPage.Name())
			}
			continue
		}

		rawCustomPage, _, err = transformPageForWeb(rawCustomPage)
		if err != nil {
			return fmt.Errorf("error transforming page: %w", err)
		}

		customPageTemplate, err := customPageSkeletonClone.Parse(`{{define "content"}}` + string(rawCustomPage) + `{{end}}`)
		if err != nil {
			return fmt.Errorf("couldn't parse custom page '%s': %w", customPage.Name(), err)
		}

		data := &customPageData{
			blogConfig:  blogConfig,
			CustomPages: customPages,
		}
		data.Hidden = headers.Hidden
		data.Title = headers.Title
		file := path.Join("pages", customPage.Name())
		customPages[index] = &customPageEntry{
			Title:    headers.Title,
			Hidden:   headers.Hidden,
			File:     file,
			data:     data,
			template: customPageTemplate,
		}
	}

	for _, page := range customPages {
		if err := writeTemplateToFile(page.template, page.data, outputDir, page.File, minifyOutput); err != nil {
			return fmt.Errorf("error writing custom page: %w", err)
		}
	}

	articles, err := os.ReadDir(filepath.Join(sourceDir, "articles"))
	if err != nil {
		return fmt.Errorf("couldn't read source articles: %w", err)
	}

	if *verbose {
		log.Println("Indexing and writing articles ...")
	}
	indexedArticles := make([]*indexedArticle, 0, len(articles))
	for _, article := range articles {
		//Other files are ignored. For example I use this to create
		//.html-draft files which are posts that I don't want to publish
		//yet, but still have in the blog source directory.
		if !strings.HasSuffix(article.Name(), ".html") {
			continue
		}

		newArticleSkeleton, err := builder.templates.Lookup("article").Clone()
		if err != nil {
			return fmt.Errorf("couldn't clone article template: %w", err)
		}

		sourcePath := filepath.Join(sourceDir, "articles", article.Name())
		headers, rawContent, err := parsePage(sourcePath)
		if err != nil {
			return fmt.Errorf("error parsing article '%s': %w", article.Name(), err)
		}

		if !includeDrafts && headers.Draft {
			if *verbose {
				fmt.Printf("Skipping article draft '%s'\n", article.Name())
			}
			continue
		}

		transformedContent, meta, err := transformPageForWeb(rawContent)
		if err != nil {
			return fmt.Errorf("error transforming article: %w", err)
		}

		specificArticleTemplate, err := newArticleSkeleton.Parse(
			`{{define "content"}}` + string(transformedContent) + `{{end}}`,
		)
		if err != nil {
			return fmt.Errorf("couldn't parse article '%s': %w", article.Name(), err)
		}
		articleData := &articlePageData{
			blogConfig:  blogConfig,
			CustomPages: customPages,
			Asciicasts:  meta.Asciicasts,
		}

		articleData.Hidden = headers.Hidden
		articleData.Title = headers.Title
		articleData.Description = headers.Description
		for tagIndex, tag := range headers.Tags {
			headers.Tags[tagIndex] = strings.ToLower(strings.TrimSpace(tag))
		}
		sort.Slice(headers.Tags, func(a, b int) bool {
			return strings.Compare(headers.Tags[a], headers.Tags[b]) == -1
		})
		articleData.Tags = headers.Tags

		articleData.RFC3339Time = headers.dateParsed.Format(time.RFC3339)
		articleData.HumanTime = headers.dateParsed.Format(blogConfig.DateFormat)
		if headers.PodcastAudio != "" {
			if strings.HasPrefix(strings.TrimPrefix(headers.PodcastAudio, "/"), "media") {
				articleData.PodcastAudio = path.Join(blogConfig.BasePath, headers.PodcastAudio)
			}
		}
		articleTargetPath := filepath.Join("articles", article.Name())

		if !articleData.Hidden {
			feedContent, err := transformPageForRSS(rawContent)
			if err != nil {
				return fmt.Errorf("error transforming content for feed: %w", err)
			}

			newIndexedArticle := &indexedArticle{
				blogConfig:   blogConfig,
				podcastAudio: articleData.PodcastAudio,
				Title:        headers.Title,
				File:         path.Join("articles", article.Name()),
				RFC3339Time:  headers.dateParsed,
				HumanTime:    articleData.HumanTime,
				FeedContent:  string(feedContent),
				Tags:         headers.Tags,
				AuthorName:   headers.Author,
				AuthorEmail:  headers.AuthorEmail,
			}
			// Fix page metadata to include correct name instead of main author.
			if newIndexedArticle.AuthorName != "" {
				newIndexedArticle.Author = newIndexedArticle.AuthorName
				articleData.Author = newIndexedArticle.AuthorName
			}

			indexedArticles = append(indexedArticles, newIndexedArticle)
		}

		if err := writeTemplateToFile(specificArticleTemplate, articleData, outputDir, articleTargetPath, minifyOutput); err != nil {
			return fmt.Errorf("error writing article: %w", err)
		}
	}

	// Sort articles to make sure the RSS feed and index have the right ordering.
	sort.Slice(indexedArticles, func(a, b int) bool {
		articleA := indexedArticles[a]
		articleB := indexedArticles[b]
		return articleB.RFC3339Time.Before(articleA.RFC3339Time)
	})

	// Collect tags from all articles for listing them in the index files.
	var tags []string
	for _, article := range indexedArticles {
	NEW_TAG_LOOP:
		for _, tag := range article.Tags {
			for _, existingTag := range tags {
				if existingTag == tag {
					continue NEW_TAG_LOOP
				}
			}

			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)

	if *verbose {
		log.Println("Writing main index files.")
	}
	indexTemplate := builder.templates.Lookup("index")
	writeIndexFiles(indexTemplate, indexedArticles, customPages, blogConfig,
		tags, "", "index.html", "index-%d.html", outputDir, minifyOutput)

	if *verbose {
		log.Println("Writing tagged index files.")
	}
	// Special Index-Files with tag-filters
	for _, tag := range tags {
		var tagFilteredArticles []*indexedArticle
	ARTICLE_LOOP:
		for _, article := range indexedArticles {
			for _, articleTag := range article.Tags {
				if articleTag == tag {
					tagFilteredArticles = append(tagFilteredArticles, article)
					continue ARTICLE_LOOP
				}
			}
		}

		writeIndexFiles(indexTemplate, tagFilteredArticles, customPages, blogConfig,
			tags, tag, "index-"+tag+".html", "index-"+tag+"-%d.html", outputDir, minifyOutput)
	}

	if *verbose {
		log.Println("Writing RSS feed.")
	}
	if err := writeRSSFeed(sourceDir, outputDir, indexedArticles, blogConfig); err != nil {
		return fmt.Errorf("error writing rss feed: %w", err)
	}

	baseCSSFile, err := skeletonFS.Open("skeletons/base.css")
	if err != nil {
		return fmt.Errorf("couldn't read base.css: %w", err)
	}
	asciinemaJSFile, err := skeletonFS.Open("skeletons/asciinema-player.min.js")
	if err != nil {
		return fmt.Errorf("couldn't read asciinema-player.min.js`: %w", err)
	}
	asciinemaCSSFile, err := skeletonFS.Open("skeletons/asciinema-player.css")
	if err != nil {
		return fmt.Errorf("couldn't read asciinema-player.css: %w", err)
	}

	if minifyOutput {
		if *verbose {
			log.Println("Copying and minifying base.css.")
		}
		baseCSSOutput, err := createFile(filepath.Join(outputDir, "base.css"))
		if err != nil {
			return err
		}
		defer baseCSSOutput.Close()

		if err := minifier.Minify("text/css", baseCSSOutput, baseCSSFile); err != nil {
			return fmt.Errorf("couldn't minify base.css: %w", err)
		}
	} else {
		if *verbose {
			log.Println("Copying base.css ...")
		}
		if err := copyDataIntoFile(baseCSSFile, filepath.Join(outputDir, "base.css")); err != nil {
			return err
		}
	}

	if *verbose {
		log.Println("Copying media directory.")
	}

	if *verbose {
		log.Println("Copying asciinema-player.min.js ...")
	}
	if err := copyDataIntoFile(asciinemaJSFile, filepath.Join(outputDir, "asciinema-player.min.js")); err != nil {
		return err
	}
	if *verbose {
		log.Println("Copying asciinema-player.css ...")
	}
	if err := copyDataIntoFile(asciinemaCSSFile, filepath.Join(outputDir, "asciinema-player.css")); err != nil {
		return err
	}

	if err := copy.Copy(filepath.Join(sourceDir, "media"), filepath.Join(outputDir, "media")); err != nil {
		return fmt.Errorf("couldn't copy media directory: %w", err)
	}

	if *verbose {
		log.Println("Writing 404.html")
	}

	return writeTemplateToFile(builder.templates.Lookup("404"), &customPageData{
		blogConfig:  blogConfig,
		CustomPages: customPages,
	}, outputDir, "404.html", minifyOutput)
}

func copyFavicon(sourceDir, outputDir string) (string, error) {
	// .ico is preferred, as it has multi resolution support.
	err := copyFileByPath(
		filepath.Join(sourceDir, "favicon.ico"),
		filepath.Join(outputDir, "favicon.ico"))
	if err == nil {
		return "favicon.ico", nil
	}

	// If we encounter any error, aside from non-existence, we early
	// exit, as trying the other format doesn't make sense.
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error copying favicon.ico: %w", err)
	}

	// Doesn't exist, fallthrough to png.

	err = copyFileByPath(
		filepath.Join(sourceDir, "favicon.png"),
		filepath.Join(outputDir, "favicon.png"))
	if err == nil {
		return "favicon.png", nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error copying favicon.png: %w", err)
	}

	return "", nil
}

// parsePage can parse both articles and custom pages.
func parsePage(pagePath string) (ArticleHeaders, []byte, error) {
	var headers ArticleHeaders
	pageFile, err := os.Open(pagePath)
	if err != nil {
		return headers, nil, fmt.Errorf("error opening article: %w", err)
	}
	defer pageFile.Close()

	pageBytes, err := io.ReadAll(pageFile)
	if err != nil {
		return headers, nil, fmt.Errorf("error reading article: %w", err)
	}

	// Prevent follow-up errors on windows.
	pageBytes = bytes.ReplaceAll(pageBytes, []byte("\r\n"), []byte("\n"))

	headerAndContent := bytes.SplitN(pageBytes, []byte("\n---\n"), 2)
	if len(headerAndContent) < 2 {
		return headers, nil, errors.New("header missing, separate with `\n---\n`")
	}
	if len(headerAndContent) > 2 {
		return headers, nil, errors.New("there must only be one header")
	}

	if err := yaml.Unmarshal(headerAndContent[0], &headers); err != nil {
		return headers, nil, fmt.Errorf("error reading headers: %w", err)
	}
	if err := headers.Parse(); err != nil {
		return headers, nil, fmt.Errorf("error parsing headers: %w", err)
	}
	return headers, headerAndContent[1], nil
}

func transformPageForRSS(post []byte) ([]byte, error) {
	// FIXME What to do with asciicasts? Convert to links?
	return post, nil
}

type transformMeta struct {
	Asciicasts []asciicastMeta
}

// transformPageForWeb transforms raw HTML into user presentable HTML for the
// webpage. This is not intended for the RSS feed.
func transformPageForWeb(post []byte) ([]byte, transformMeta, error) {
	var meta transformMeta

	reader := bytes.NewReader(post)
	writer := bytes.NewBuffer(make([]byte, 0, len(post)+1048))
	tokenizer := html.NewTokenizer(reader)
	handleErr := func(err error) ([]byte, transformMeta, error) {
		if errors.Is(err, io.EOF) {
			return writer.Bytes(), meta, nil
		}
		return nil, meta, err
	}

	for {
		tokenType := tokenizer.Next()
		token := tokenizer.Token()
		switch tokenType {
		case html.ErrorToken:
			return handleErr(tokenizer.Err())
		case html.StartTagToken:
			switch token.Data {
			case "script":
				writer.WriteString(token.String())
				tokenizer.Next()
				token := tokenizer.Token()
				writer.WriteString(token.Data)
				continue
			case "h2", "h3", "h4", "h5", "h6":
				if err := transformHeading(tokenizer, token, writer); err != nil {
					return handleErr(err)
				}
				continue
			case "img":
				if err := transformImage(token, writer); err != nil {
					return handleErr(err)
				}
				continue
			}
		case html.SelfClosingTagToken:
			// Some tags are self-closing, such as "img". Meaning it doesn't matter
			// whether you put "<img>" or "</img>". However, the tokenizer will
			// still output a different token type, as the parsing isn't semantic,
			// so we treat both types, as browsers are lenient.
			switch token.Data {
			case "asciicast":
				if asciicastMeta, err := transformAsciicast(token, writer); err != nil {
					return handleErr(err)
				} else {
					meta.Asciicasts = append(meta.Asciicasts, asciicastMeta)
				}
				continue
			case "img":
				if err := transformImage(token, writer); err != nil {
					return handleErr(err)
				}
				continue
			}
		}

		writer.WriteString(token.String())
	}
}

var (
	idCharsToRemove  = regexp.MustCompile("[^a-z0-9 ]")
	idCharsToReplace = regexp.MustCompile("[ ]")
)

func convertToElementId(text string) string {
	id := strings.ToLower(text)
	id = idCharsToRemove.ReplaceAllLiteralString(id, "")
	id = idCharsToReplace.ReplaceAllLiteralString(id, "_")
	return id
}

func next(tokenizer *html.Tokenizer) (html.TokenType, html.Token, error) {
	tokenType := tokenizer.Next()
	token := tokenizer.Token()
	if tokenType == html.ErrorToken {
		return tokenType, token, tokenizer.Err()
	}
	return tokenType, token, nil
}

func attr(token html.Token, key string) (string, bool) {
	for _, attr := range token.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

type asciicastMeta struct {
	Id  string
	Src string
}

func transformAsciicast(asciicastToken html.Token, writer *bytes.Buffer) (asciicastMeta, error) {
	var meta asciicastMeta
	src, _ := attr(asciicastToken, "src")
	if src == "" {
		return meta, fmt.Errorf("source empty")
	}

	hash := fnv.New32()
	_, err := hash.Write([]byte(src))
	if err != nil {
		return meta, fmt.Errorf("error hashing path: %w", err)
	}
	srcHash := fmt.Sprintf("%x", hash.Sum(nil))

	meta.Id = string(srcHash)
	meta.Src = src

	writer.WriteString(fmt.Sprintf(`<div id="%s"></div>`, meta.Id))

	return meta, nil
}

func transformImage(imageToken html.Token, writer *bytes.Buffer) error {
	_, hasWidth := attr(imageToken, "width")
	_, hasHeight := attr(imageToken, "height")

	loading, _ := attr(imageToken, "loading")
	if loading == "lazy" {
		if !hasWidth || !hasHeight {
			return fmt.Errorf("image tag '%s' is set to load lazy, but doesn't have a width and height", imageToken.String())
		}
	}

	// If width and height are available, we default to lazy loading, if eager
	// isn't defined explicitly.
	if loading == "" && hasWidth && hasHeight {
		imageToken.Attr = append(imageToken.Attr, html.Attribute{Key: "loading", Val: "lazy"})
	}

	writer.WriteString(imageToken.String())
	return nil
}

func transformHeading(tokenizer *html.Tokenizer, headingOpen html.Token, writer *bytes.Buffer) error {
	var lastText string
	for {
		tokenType, token, err := next(tokenizer)
		if err != nil {
			return err
		}

		switch tokenType {
		case html.TextToken:
			lastText = token.String()
		case html.EndTagToken:
			if lastText != "" {
				id := convertToElementId(lastText)
				headingOpen.Attr = append(headingOpen.Attr, html.Attribute{Key: "id", Val: id})
				writer.WriteString(headingOpen.String())
				writer.WriteString(lastText)
				writer.WriteString(fmt.Sprintf(`<a class="h-a" href="#%s">#</a>`, id))
				lastText = ""
			}

			writer.WriteString(token.String())
			return nil
		default:
			writer.WriteString(token.String())
		}
	}
}

// cleanup deletes previously generated files.
func cleanup(output string) error {
	if *verbose {
		log.Println("Clearing output directory.")
	}

	if err := removeAll(
		filepath.Join(output, "media"),
		filepath.Join(output, "articles"),
		filepath.Join(output, "pages"),
		filepath.Join(output, "favicon.ico"),
		filepath.Join(output, "favicon.png"),
		filepath.Join(output, "base.css"),
		filepath.Join(output, "404.html"),
		filepath.Join(output, "feed.xml"),
	); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(output, "index*.html"))
	if err != nil {
		return fmt.Errorf("couldn't delete old index*.html files: %w", err)
	}
	for _, indexToDelete := range files {
		os.Remove(indexToDelete)
	}

	return nil
}

// writeIndexFiles writes paginated index files. It supports both tagged
// index files and untagged (default) index files.
func writeIndexFiles(
	indexTemplate *template.Template,
	indexedArticles []*indexedArticle,
	customPages []*customPageEntry,
	loadedPageConfig blogConfig,
	tags []string,
	filterTag string,
	firstIndexName string,
	indexNameTemplate string,
	outputFolder string,
	minifyOutput bool,
) error {
	currentPageNumber := 1
	lastPageNumber := len(indexedArticles) / loadedPageConfig.MaxIndexEntries
	if len(indexedArticles)%loadedPageConfig.MaxIndexEntries != 0 {
		lastPageNumber++
	}

	for i := 1; i <= len(indexedArticles); i += loadedPageConfig.MaxIndexEntries {
		var pageName string
		if currentPageNumber == 1 {
			pageName = firstIndexName
		} else {
			pageName = fmt.Sprintf(indexNameTemplate, currentPageNumber)
		}
		data := &indexData{
			blogConfig:       loadedPageConfig,
			Tags:             tags,
			FilterTag:        filterTag,
			CustomPages:      customPages,
			IndexedArticles:  indexedArticles[i-1 : min(i-1+loadedPageConfig.MaxIndexEntries, len(indexedArticles))],
			PageNameTemplate: indexNameTemplate,
			CurrentPageNum:   currentPageNumber,
			FirstPage:        firstIndexName,
			LastPageNum:      lastPageNumber,
		}
		if i+loadedPageConfig.MaxIndexEntries <= len(indexedArticles) {
			data.NextPageNum = currentPageNumber + 1
		}
		if currentPageNumber > 1 {
			data.PrevPageNum = currentPageNumber - 1
		}

		if err := writeTemplateToFile(indexTemplate, data, outputFolder, pageName, minifyOutput); err != nil {
			return nil
		}
		currentPageNumber++
	}

	return nil
}

func writeRSSFeed(sourceFolder, outputFolder string, articles []*indexedArticle, loadedPageConfig blogConfig) error {
	var mainAuthor *feeds.Author
	if loadedPageConfig.Email != "" {
		mainAuthor = &feeds.Author{
			Name:  loadedPageConfig.Author,
			Email: loadedPageConfig.Email,
		}
	}
	feed := &feeds.Feed{
		Title:       loadedPageConfig.SiteName,
		Description: loadedPageConfig.Description,
		Author:      mainAuthor,
	}
	if loadedPageConfig.URL != "" {
		feed.Link = &feeds.Link{Href: loadedPageConfig.URL}
	}
	if loadedPageConfig.CreationDate != "" {
		var err error
		feed.Created, err = time.Parse(time.RFC3339, loadedPageConfig.CreationDate)
		if err != nil {
			return err
		}
	}

	for _, article := range articles {
		newFeedItem := &feeds.Item{
			Title:       article.Title,
			Author:      mainAuthor,
			Content:     article.FeedContent,
			Description: article.Description,
			Created:     article.RFC3339Time,
		}
		if article.AuthorEmail != "" || article.AuthorName != "" {
			articleAuthor := &feeds.Author{
				Name: article.AuthorName,
			}
			if article.AuthorEmail != "" {
				articleAuthor.Email = article.AuthorEmail
			} else if mainAuthor != nil {
				articleAuthor.Email = mainAuthor.Email
			}
			newFeedItem.Author = articleAuthor
		}
		if article.podcastAudio != "" {
			audioFilepath := filepath.Join(sourceFolder, article.podcastAudio)
			audioFile, err := os.Stat(audioFilepath)
			if err != nil {
				return fmt.Errorf("couldn't read podcast audio file '%s': %w", audioFilepath, err)
			}
			audioURL, err := joinURLParts(feed.Link.Href, article.podcastAudio)
			if err != nil {
				return fmt.Errorf("couldn't generate audio URL: %w", err)
			}
			newFeedItem.Enclosure = &feeds.Enclosure{
				Type:   "audio/mp3",
				Length: strconv.FormatInt(audioFile.Size(), 10),
				Url:    audioURL,
			}
		}
		feed.Items = append(feed.Items, newFeedItem)
		if loadedPageConfig.URL != "" {
			articleURL, err := joinURLParts(loadedPageConfig.URL, article.File)
			if err != nil {
				return fmt.Errorf("couldn't generate article URL: %w", err)
			}
			newFeedItem.Link = &feeds.Link{Href: articleURL}
		}
	}

	rssFile, err := createFile(filepath.Join(outputFolder, "feed.xml"))
	if err != nil {
		return err
	}
	defer rssFile.Close()

	rssData, err := feed.ToRss()
	if err != nil {
		return fmt.Errorf("couldn't generate RSS feed: %w", err)
	}
	_, err = rssFile.WriteString(rssData)
	if err != nil {
		return fmt.Errorf("couldn't write RSS feed: %w", err)
	}

	return nil
}

// joinURLParts puts together two URL pieces without duplicating separators
// or removing separators. Before, this was done by path.Join directly which
// caused the resulting URL to be missing a forward slash behind the protocol.
// Using filepath.Join would cause incorrect separators on windows, as URLs
// should always use forward slashes, but windows uses backward slashes.
func joinURLParts(partOne, partTwo string) (string, error) {
	url, err := url.Parse(partOne)
	if err != nil {
		return "", err
	}

	url.Path = path.Join(url.Path, partTwo)
	return url.String(), nil
}

type blogConfig struct {
	BasePath string
	// Hidden will not show any links to the given page. This works for both
	// custom pages and articles.
	Hidden bool
	// Title is the page titel, which differs from the SiteName.
	Title               string
	SiteName            string
	Author              string
	URL                 string
	Description         string
	DateFormat          string
	Email               string
	CreationDate        string
	UtterancesRepo      string
	MaxIndexEntries     int
	AddOptionalMetaData bool
	Favicon             string
}

type customPageEntry struct {
	Title string
	File  string
	// Hidden will not show any links to the given page. This works for both
	// custom pages and articles.
	Hidden   bool
	data     *customPageData
	template *template.Template
}

type articlePageData struct {
	blogConfig
	// Time article was published in RFC3339 format.
	RFC3339Time string
	// HumanTime is a human readable time format.
	HumanTime string
	// PodcastAudio file link
	PodcastAudio string
	// Tags for metadata
	Tags []string
	// CustomPages are listed right of the default pages in the site navbar /
	// header.
	CustomPages []*customPageEntry
	// Asciicast defines whether one or more asciicast elements are present on
	// the page, automatically causing the generator to add the required scripts
	// and stylesheets.
	Asciicasts []asciicastMeta
}

type customPageData struct {
	blogConfig

	// CustomPages are listed right of the default pages in the site navbar /
	// header.
	CustomPages []*customPageEntry
}

type indexData struct {
	blogConfig
	// Tags are all available tags used accross all posts
	Tags []string
	// FilterTag that is currently filtered for
	FilterTag string
	// CustomPages are listed right of the default pages in the site navbar /
	// header.
	CustomPages []*customPageEntry
	// IndexedArticles are the articles to display.
	IndexedArticles []*indexedArticle

	PageNameTemplate string

	FirstPage string

	CurrentPageNum int
	PrevPageNum    int
	NextPageNum    int
	LastPageNum    int
}

type indexedArticle struct {
	blogConfig
	AuthorName   string
	AuthorEmail  string
	Title        string
	File         string
	RFC3339Time  time.Time
	podcastAudio string
	HumanTime    string
	FeedContent  string
	Tags         []string
}
