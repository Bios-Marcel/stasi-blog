package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Bios-Marcel/feeds"
	"github.com/otiai10/copy"
)

//go:embed skeletons/*
var skeletonFS embed.FS

func build(sourceFolder, output, config string, minifyOutput bool) error {
	if err := cleanup(output); err != nil {
		return fmt.Errorf("error performing cleanup: %w", err)
	}

	//Create empty directories
	err := createDirectories(
		filepath.Join(output, "media"),
		filepath.Join(output, "articles"),
		filepath.Join(output, "pages"),
	)
	if err != nil {
		return fmt.Errorf("error preparing target folder structure: %w", err)
	}

	loadedPageConfig := pageConfig{
		DateFormat:      "2 January 2006",
		UseFavicon:      true,
		MaxIndexEntries: 10,
	}
	var configPath string
	if config != "" {
		configPath = config
	} else {
		configPath = filepath.Join(sourceFolder, "config.json")
	}
	configFile, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("error loading config '%s': %w", configPath, err)
	}

	if err := json.NewDecoder(configFile).Decode(&loadedPageConfig); err != nil {
		log.Fatalf("Error decoding config: %s\n", err)
	}
	if loadedPageConfig.BasePath != "" {
		//Making sure there's not too many or too little slashes ;)
		loadedPageConfig.BasePath = "/" + strings.Trim(loadedPageConfig.BasePath, `/\`)
	}

	if loadedPageConfig.UseFavicon {
		// .ico is preferred, as it has multi resolution support.
		if err := copyFileByPath(
			filepath.Join(sourceFolder, "favicon.ico"),
			filepath.Join(output, "favicon.ico")); err != nil {
			if !os.IsNotExist(err) {
				log.Println("error copying favicon.ico:", err)
			}
			if err := copyFileByPath(
				filepath.Join(sourceFolder, "favicon.png"),
				filepath.Join(output, "favicon.png")); err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("error copying favicon.ico: %w", err)
				} else {
					return fmt.Errorf("favicon.ico/png couldn't be found. If you don't want to use a favicon, set 'UseFavicon' to 'false'")
				}
			}
		}
	}

	parsedTemplates, err := template.New("").
		Funcs(template.FuncMap{
			"sub": func(a, b int) int {
				return a - b
			},
			"add": func(a, b int) int {
				return a + b
			},
		}).
		ParseFS(skeletonFS, "skeletons/*.html")
	if err != nil {
		return fmt.Errorf("couldn't parse HTML templates: %w", err)
	}

	customPageFiles, err := os.ReadDir(filepath.Join(sourceFolder, "pages"))
	if err != nil {
		return fmt.Errorf("couldn't handle pages directory: %w", err)
	}

	customPageTemplates := make(map[string]*template.Template, len(customPageFiles))
	customPages := make([]*customPageEntry, len(customPageFiles))
	for index, customPage := range customPageFiles {
		customPageSkeletonClone, err := parsedTemplates.Lookup("page").Clone()
		if err != nil {
			return fmt.Errorf("couldn't clone 'page' template: %w", err)
		}

		sourcePath := filepath.Join(sourceFolder, "pages", customPage.Name())
		customPageTemplate, err := customPageSkeletonClone.ParseFiles(sourcePath)
		if err != nil {
			return fmt.Errorf("couldn't parse custom page '%s': %w", customPage, err)
		}

		customPageTemplates[customPage.Name()] = customPageTemplate

		title, err := templateToString(customPageTemplate.Lookup("title"))
		if err != nil {
			return err
		}
		customPages[index] = &customPageEntry{
			Title: title,
			File:  path.Join("pages", customPage.Name()),
		}
	}

	articles, err := os.ReadDir(filepath.Join(sourceFolder, "articles"))
	if err != nil {
		return fmt.Errorf("couldn't read source articles: %w", err)
	}

	if *verbose {
		log.Println("Indexing and writing articles.")
	}
	indexedArticles := make([]*indexedArticle, 0, len(articles))
	for _, article := range articles {
		//Other files are ignored. For example I use this to create
		//.html-draft files which are posts that I don't want to publish
		//yet, but still have in the blog source directory.
		if !strings.HasSuffix(article.Name(), ".html") {
			continue
		}

		newArticleSkeleton, err := parsedTemplates.Lookup("article").Clone()
		if err != nil {
			return fmt.Errorf("couldn't clone article template: %w", err)
		}
		sourcePath := filepath.Join(sourceFolder, "articles", article.Name())
		specificArticleTemplate, err := newArticleSkeleton.ParseFiles(sourcePath)
		if err != nil {
			return fmt.Errorf("couldn't parse article '%s': %w", article, err)
		}
		articleData := &articlePageData{
			pageConfig:  loadedPageConfig,
			CustomPages: customPages,
		}

		articleData.Description, err = templateToOptionalString(specificArticleTemplate.Lookup("description"))
		if err != nil {
			return err
		}
		tagString, err := templateToOptionalString(specificArticleTemplate.Lookup("tags"))
		if err != nil {
			return err
		}
		tagString = strings.TrimSpace(tagString)
		var tags []string
		//Tags are optional
		if tagString != "" {
			tags = strings.Split(tagString, ",")
			for tagIndex, tag := range tags {
				tags[tagIndex] = strings.ToLower(strings.TrimSpace(tag))
			}
			sort.Slice(tags, func(a, b int) bool {
				return strings.Compare(tags[a], tags[b]) == -1
			})
		}
		articleData.Tags = tags
		dateAsString, err := templateToString(specificArticleTemplate.Lookup("date"))
		if err != nil {
			return err
		}
		publishTime, err := time.Parse("2006-01-02", dateAsString)
		if err != nil {
			return fmt.Errorf("couldn't parse date '%s': %w", dateAsString, err)
		}
		articleData.RFC3339Time = publishTime.Format(time.RFC3339)
		articleData.HumanTime = publishTime.Format(loadedPageConfig.DateFormat)
		articleData.PodcastAudio, err = templateToOptionalString(specificArticleTemplate.Lookup("podcast-audio"))
		if err != nil {
			return err
		}
		if articleData.PodcastAudio != "" {
			if strings.HasPrefix(strings.TrimPrefix(articleData.PodcastAudio, "/"), "media") {
				articleData.PodcastAudio = path.Join(loadedPageConfig.BasePath, articleData.PodcastAudio)
			}
		}
		articleTargetPath := filepath.Join("articles", article.Name())

		title, err := templateToString(specificArticleTemplate.Lookup("title"))
		if err != nil {
			return err
		}
		content, err := templateToString(specificArticleTemplate.Lookup("content"))
		if err != nil {
			return err
		}
		author, err := templateToOptionalString(specificArticleTemplate.Lookup("author"))
		if err != nil {
			return err
		}
		author = strings.TrimSpace(author)
		authorEmail, err := templateToOptionalString(specificArticleTemplate.Lookup("author-email"))
		if err != nil {
			return err
		}
		authorEmail = strings.TrimSpace(authorEmail)

		newIndexedArticle := &indexedArticle{
			pageConfig:   loadedPageConfig,
			podcastAudio: articleData.PodcastAudio,
			Title:        title,
			File:         path.Join("articles", article.Name()),
			RFC3339Time:  publishTime,
			HumanTime:    publishTime.Format(loadedPageConfig.DateFormat),
			Content:      content,
			Tags:         tags,
			AuthorName:   author,
			AuthorEmail:  authorEmail,
		}
		//Fix page metadata to include correct name instead of main author.
		if newIndexedArticle.AuthorName != "" {
			newIndexedArticle.Author = newIndexedArticle.AuthorName
			articleData.Author = newIndexedArticle.AuthorName
		}

		indexedArticles = append(indexedArticles, newIndexedArticle)

		writeTemplateToFile(specificArticleTemplate, articleData, output, articleTargetPath, minifyOutput)
	}

	//Sort articles to make sure the RSS feed and index have the right ordering.
	sort.Slice(indexedArticles, func(a, b int) bool {
		articleA := indexedArticles[a]
		articleB := indexedArticles[b]
		return articleB.RFC3339Time.Before(articleA.RFC3339Time)
	})

	//Collect tags from all articles for listing them in the index files.
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

	if *verbose {
		log.Println("Writing custom pages.")
	}
	//We first populate the custom page array so that the pages all have a correct menu header.
	for fileName, customPageTemplate := range customPageTemplates {
		writeTemplateToFile(customPageTemplate, &customPageData{
			pageConfig:  loadedPageConfig,
			CustomPages: customPages,
		}, output, filepath.Join("pages", fileName), minifyOutput)
	}

	indexTemplate := parsedTemplates.Lookup("index")

	if *verbose {
		log.Println("Writing main index files.")
	}
	writeIndexFiles(indexTemplate, indexedArticles, customPages, loadedPageConfig,
		tags, "", "index.html", "index-%d.html", output, minifyOutput)

	if *verbose {
		log.Println("Writing tagged index files.")
	}
	//Special Index-Files with tag-filters
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

		writeIndexFiles(indexTemplate, tagFilteredArticles, customPages, loadedPageConfig,
			tags, tag, "index-"+tag+".html", "index-"+tag+"-%d.html", output, minifyOutput)
	}

	if *verbose {
		log.Println("Writing RSS feed.")
	}
	if err := writeRSSFeed(sourceFolder, output, indexedArticles, loadedPageConfig); err != nil {
		return fmt.Errorf("error writing rss feed: %w", err)
	}

	baseCSSFile, err := skeletonFS.Open("skeletons/base.css")
	if err != nil {
		return fmt.Errorf("couldn't read base.css: %w", err)
	}

	if minifyOutput {
		if *verbose {
			log.Println("Copying and minifying base.css.")
		}
		baseCSSOutput, err := createFile(filepath.Join(output, "base.css"))
		if err != nil {
			return err
		}

		if err := minifier.Minify("text/css", baseCSSOutput, baseCSSFile); err != nil {
			return fmt.Errorf("couldn't minify base.css: %w", err)
		}
	} else {
		if *verbose {
			log.Println("Copying base.css.")
		}
		if err := copyDataIntoFile(baseCSSFile, filepath.Join(output, "base.css")); err != nil {
			return err
		}
	}

	if *verbose {
		log.Println("Copying media directory.")
	}

	if err := copy.Copy(filepath.Join(sourceFolder, "media"), filepath.Join(output, "media")); err != nil {
		return fmt.Errorf("couldn't copy media directory: %w", err)
	}

	if *verbose {
		log.Println("Writing 404.html")
	}

	return writeTemplateToFile(parsedTemplates.Lookup("404"), &customPageData{
		pageConfig:  loadedPageConfig,
		CustomPages: customPages,
	}, output, "404.html", minifyOutput)
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
	loadedPageConfig pageConfig,
	tags []string,
	filterTag string,
	firstIndexName string,
	indexNameTemplate string,
	outputFolder string,
	minifyOutput bool) error {

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
			pageConfig:       loadedPageConfig,
			Tags:             tags,
			FilterTag:        filterTag,
			CustomPages:      customPages,
			IndexedArticles:  indexedArticles[i-1 : minInt(i-1+loadedPageConfig.MaxIndexEntries, len(indexedArticles))],
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

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func writeRSSFeed(sourceFolder, outputFolder string, articles []*indexedArticle, loadedPageConfig pageConfig) error {
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
			Content:     article.Content,
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

type pageConfig struct {
	BasePath            string
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
	UseFavicon          bool
}

type customPageEntry struct {
	Title string
	File  string
}

type articlePageData struct {
	pageConfig
	//Time article was published in RFC3339 format.
	RFC3339Time string
	//HumanTime is a human readable time format.
	HumanTime string
	//PodcastAudio file link
	PodcastAudio string
	//Tags for metadata
	Tags []string
	//CustomPages are pages listed in the header next to "Home"
	CustomPages []*customPageEntry
}

type customPageData struct {
	pageConfig
	//CustomPages are pages listed in the header next to "Home"
	CustomPages []*customPageEntry
}

type indexData struct {
	pageConfig
	//Tags are all available tags used accross all posts
	Tags []string
	//FilterTag that is currently filtered for
	FilterTag string
	//CustomPages are pages listed in the header next to "Home"
	CustomPages []*customPageEntry
	//IndexedArticles are the articles to display.
	IndexedArticles []*indexedArticle

	PageNameTemplate string

	FirstPage string

	CurrentPageNum int
	PrevPageNum    int
	NextPageNum    int
	LastPageNum    int
}

func templateToString(temp *template.Template) (string, error) {
	buffer := &bytes.Buffer{}
	if err := temp.Execute(buffer, nil); err != nil {
		return "", fmt.Errorf("couldn't execute template '%s': %w", temp.Name(), err)
	}
	return buffer.String(), nil
}

func templateToOptionalString(temp *template.Template) (string, error) {
	if temp == nil {
		return "", nil
	}

	return templateToString(temp)
}

type indexedArticle struct {
	pageConfig
	AuthorName   string
	AuthorEmail  string
	Title        string
	File         string
	RFC3339Time  time.Time
	podcastAudio string
	HumanTime    string
	Content      string
	Tags         []string
}
