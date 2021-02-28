package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/otiai10/copy"
)

//go:embed skeletons/*
var skeletonFS embed.FS

func build(sourceFolder, output, config string, minifyOutput bool) {
	var configPath string
	if config != "" {
		configPath = config
	} else {
		configPath = filepath.Join(sourceFolder, "config.json")
	}

	configFile, openError := os.Open(configPath)
	if openError != nil {
		log.Fatalf("Error loading config: %s\n", openError)
	}

	if *verbose {
		showInfo("Clearing output directory.")
	}
	//Delete old data
	os.RemoveAll(filepath.Join(output, "media"))
	os.RemoveAll(filepath.Join(output, "articles"))
	os.RemoveAll(filepath.Join(output, "pages"))
	os.Remove(filepath.Join(output, "favicon.ico"))
	os.Remove(filepath.Join(output, "favicon.png"))
	os.Remove(filepath.Join(output, "base.css"))
	os.Remove(filepath.Join(output, "404.html"))
	os.Remove(filepath.Join(output, "feed.xml"))
	files, globError := filepath.Glob(filepath.Join(output, "index*.html"))
	if globError != nil {
		exitWithError("Couldn't delete old index*.html files", globError.Error())
	}
	for _, indexToDelete := range files {
		os.Remove(indexToDelete)
	}

	//Create empty directories
	createDirectory(filepath.Join(output, "media"))
	createDirectory(filepath.Join(output, "articles"))
	createDirectory(filepath.Join(output, "pages"))

	loadedPageConfig := pageConfig{
		DateFormat:      "2 January 2006",
		UseFavicon:      true,
		MaxIndexEntries: 10,
	}
	configDecodeError := json.NewDecoder(configFile).Decode(&loadedPageConfig)
	if configDecodeError != nil {
		log.Fatalf("Error decoding config: %s\n", configDecodeError)
	}
	if loadedPageConfig.BasePath != "" {
		//Making sure there's not too many or too little slashes ;)
		loadedPageConfig.BasePath = "/" + strings.Trim(loadedPageConfig.BasePath, "/\\")
	}

	if loadedPageConfig.UseFavicon {
		faviconIcoSourcePath := filepath.Join(sourceFolder, "favicon.ico")
		_, faviconIcoErr := os.Stat(faviconIcoSourcePath)
		if faviconIcoErr != nil {
			faviconPngSourcePath := filepath.Join(sourceFolder, "favicon.png")
			_, faviconPngErr := os.Stat(faviconPngSourcePath)
			if faviconPngErr != nil {
				log.Fatalln("favicon.ico/png couldn't be found. If you don't want to use a favicon, set 'UseFavicon' to 'false'.")
			} else {
				copyFileByPath(faviconPngSourcePath, filepath.Join(output, "favicon.png"))
			}
		} else {
			copyFileByPath(faviconIcoSourcePath, filepath.Join(output, "favicon.ico"))
		}
	}

	parsedTemplates, parseError := template.New("").
		Funcs(template.FuncMap{
			"sub": func(a, b int) int {
				return a - b
			},
			"add": func(a, b int) int {
				return a + b
			},
		}).
		ParseFS(skeletonFS, "skeletons/*.html")
	if parseError != nil {
		exitWithError("Couldn't parse HTML templates", parseError.Error())
	}

	customPageFiles, pagesFolderError := ioutil.ReadDir(filepath.Join(sourceFolder, "pages"))
	if pagesFolderError != nil {
		if os.IsNotExist(pagesFolderError) {
			if *verbose {
				showWarning("pages directory couldn't be found and therefore couldn't be handled.")
			}
		} else {
			exitWithError("Couldn't handle pages directory", pagesFolderError.Error())
		}
	}

	customPageTemplates := make(map[string]*template.Template)
	var customPages []*customPageEntry
	for _, customPage := range customPageFiles {
		customPageSkeletonClone, cloneError := parsedTemplates.Lookup("page").Clone()
		if cloneError != nil {
			exitWithError("Couldn't clone page template", cloneError.Error())
		}

		sourcePath := filepath.Join(sourceFolder, "pages", customPage.Name())
		customPageTemplate, parseError := customPageSkeletonClone.ParseFiles(sourcePath)
		if parseError != nil {
			exitWithError(fmt.Sprintf("Couldn't parse custom page '%s'", customPage), cloneError.Error())
		}

		customPageTemplates[customPage.Name()] = customPageTemplate

		customPages = append(customPages, &customPageEntry{
			Title: templateToString(customPageTemplate.Lookup("title")),
			File:  path.Join("pages", customPage.Name()),
		})
	}

	articles, articlesReadError := ioutil.ReadDir(filepath.Join(sourceFolder, "articles"))
	if articlesReadError != nil {
		exitWithError("Couldn't read source articles", articlesReadError.Error())
	}

	if *verbose {
		showInfo("Indexing and writing articles.")
	}
	var indexedArticles []*indexedArticle
	for _, article := range articles {
		//Other files are ignored. For example I use this to create
		//.html-draft files which are posts that I don't want to publish
		//yet, but still have in the blog source directory.
		if !strings.HasSuffix(article.Name(), ".html") {
			continue
		}

		newArticleSkeleton, cloneError := parsedTemplates.Lookup("article").Clone()
		if cloneError != nil {
			exitWithError("Couldn't clone article template", cloneError.Error())
		}
		sourcePath := filepath.Join(sourceFolder, "articles", article.Name())
		specificArticleTemplate, parseError := newArticleSkeleton.ParseFiles(sourcePath)
		if parseError != nil {
			exitWithError(fmt.Sprintf("Couldn't parse article '%s'", article), cloneError.Error())
		}
		articleData := &articlePageData{
			pageConfig:  loadedPageConfig,
			CustomPages: customPages,
		}
		articleData.Description = templateToOptionalString(specificArticleTemplate.Lookup("description"))
		tagString := strings.TrimSpace(templateToOptionalString(specificArticleTemplate.Lookup("tags")))
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
		dateAsString := templateToString(specificArticleTemplate.Lookup("date"))
		publishTime, timeParseError := time.Parse("2006-01-02", dateAsString)
		if timeParseError != nil {
			exitWithError(fmt.Sprintf("Couldn't parse date '%s'", dateAsString), timeParseError.Error())
		}
		articleData.RFC3339Time = publishTime.Format(time.RFC3339)
		articleData.HumanTime = publishTime.Format(loadedPageConfig.DateFormat)
		articleData.PodcastAudio = templateToOptionalString(specificArticleTemplate.Lookup("podcast-audio"))
		if articleData.PodcastAudio != "" {
			if strings.HasPrefix(strings.TrimPrefix(articleData.PodcastAudio, "/"), "media") {
				articleData.PodcastAudio = path.Join(loadedPageConfig.BasePath, articleData.PodcastAudio)
			}
		}
		articleTargetPath := filepath.Join("articles", article.Name())

		writeTemplateToFile(specificArticleTemplate, articleData, output, articleTargetPath, minifyOutput)

		newIndexedArticle := &indexedArticle{
			pageConfig:   loadedPageConfig,
			podcastAudio: templateToOptionalString(specificArticleTemplate.Lookup("podcast-audio")),
			Title:        templateToString(specificArticleTemplate.Lookup("title")),
			File:         path.Join("articles", article.Name()),
			RFC3339Time:  publishTime,
			HumanTime:    publishTime.Format(loadedPageConfig.DateFormat),
			Content:      templateToString(specificArticleTemplate.Lookup("content")),
			Tags:         tags,
		}
		indexedArticles = append(indexedArticles, newIndexedArticle)
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
		showInfo("Writing custom pages.")
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
		showInfo("Writing main index files.")
	}
	writeIndexFiles(indexTemplate, indexedArticles, customPages, loadedPageConfig,
		tags, "", "index.html", "index-%d.html", output, minifyOutput)

	if *verbose {
		showInfo("Writing tagged index files.")
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

		writeIndexFiles(indexTemplate, indexedArticles, customPages, loadedPageConfig,
			tags, tag, "index-tag-"+tag+".html", "index-tag-"+tag+"-%d.html", output, minifyOutput)
	}

	if *verbose {
		showInfo("Writing RSS feed.")
	}
	writeRSSFeed(sourceFolder, output, indexedArticles, loadedPageConfig)

	baseCSSFile, fsError := skeletonFS.Open("skeletons/base.css")
	if fsError != nil {
		exitWithError("Couldn't read base.css", fsError.Error())
	}

	if minifyOutput {
		baseCSSOutput := createFile(filepath.Join(output, "base.css"))
		if *verbose {
			showInfo("Copying and minifying base.css.")
		}
		minifyError := minifier.Minify("text/css", baseCSSOutput, baseCSSFile)
		if minifyError != nil {
			exitWithError("Couldn't minify base.css", minifyError.Error())
		}
	} else {
		if *verbose {
			showInfo("Copying base.css.")
		}
		copyDataIntoFile(baseCSSFile, filepath.Join(output, "base.css"))
	}

	if *verbose {
		showInfo("Copying media directory.")
	}
	mediaCopyError := copy.Copy(filepath.Join(sourceFolder, "media"), filepath.Join(output, "media"))
	if mediaCopyError != nil {
		if os.IsNotExist(mediaCopyError) {
			if *verbose {
				showWarning("media directory couldn't be found and therefore couldn't be copied.")
			}
		} else {
			exitWithError("Couldn't copy media directory", pagesFolderError.Error())
		}
	}

	if *verbose {
		showInfo("Writing 404.html")
	}
	writeTemplateToFile(parsedTemplates.Lookup("404"), &customPageData{
		pageConfig:  loadedPageConfig,
		CustomPages: customPages,
	}, output, "404.html", minifyOutput)
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
	minifyOutput bool) {
	for i := 1; i <= len(indexedArticles); i += loadedPageConfig.MaxIndexEntries {
		var pageName string
		if i == 1 {
			pageName = firstIndexName
		} else {
			pageName = fmt.Sprintf(indexNameTemplate, i)
		}
		data := &indexData{
			pageConfig:       loadedPageConfig,
			Tags:             tags,
			FilterTag:        filterTag,
			CustomPages:      customPages,
			IndexedArticles:  indexedArticles[i-1 : i-1+loadedPageConfig.MaxIndexEntries],
			PageNameTemplate: indexNameTemplate,
			CurrentPageNum:   i,
			FirstPage:        firstIndexName,
		}
		if i+loadedPageConfig.MaxIndexEntries <= len(indexedArticles) {
			data.NextPageNum = i + 1
		}
		if i > 1 {
			data.PrevPageNum = i - 1
		}
		data.LastPageNum = len(indexedArticles) / loadedPageConfig.MaxIndexEntries
		if len(indexedArticles)%loadedPageConfig.MaxIndexEntries != 0 {
			data.LastPageNum++
		}

		writeTemplateToFile(indexTemplate, data, outputFolder, pageName, minifyOutput)
	}
}

func writeRSSFeed(sourceFolder, outputFolder string, articles []*indexedArticle, loadedPageConfig pageConfig) {
	var author *feeds.Author
	if loadedPageConfig.Author != "" || loadedPageConfig.Email != "" {
		author = &feeds.Author{
			Name:  loadedPageConfig.Author,
			Email: loadedPageConfig.Email,
		}
	}
	feed := &feeds.Feed{
		Title:       loadedPageConfig.SiteName,
		Description: loadedPageConfig.Description,
		Author:      author,
	}
	if loadedPageConfig.URL != "" {
		feed.Link = &feeds.Link{Href: loadedPageConfig.URL}
	}
	if loadedPageConfig.CreationDate != "" {
		feed.Created = timeFromRFC3339(loadedPageConfig.CreationDate)
	}

	for _, article := range articles {
		newFeedItem := &feeds.Item{
			Title: article.Title,
			//Causes feed to be invalid.
			//Author:      author,
			Content:     article.Content,
			Description: article.Description,
			Created:     article.RFC3339Time,
		}
		if article.podcastAudio != "" {
			audioFilepath := filepath.Join(sourceFolder, article.podcastAudio)
			audioFile, statError := os.Stat(audioFilepath)
			if statError != nil {
				exitWithError(fmt.Sprintf("Couldn't read podcast audio file '%s'", audioFilepath), statError.Error())
			}
			audioURL, joinError := joinURLParts(feed.Link.Href, article.podcastAudio)
			if joinError != nil {
				exitWithError("Couldn't generate audio URL", joinError.Error())
			}
			newFeedItem.Enclosure = &feeds.Enclosure{
				Type:   "audio/mp3",
				Length: strconv.FormatInt(audioFile.Size(), 10),
				Url:    audioURL,
			}
		}
		feed.Items = append(feed.Items, newFeedItem)
		if loadedPageConfig.URL != "" {
			articleURL, joinError := joinURLParts(loadedPageConfig.URL, article.File)
			if joinError != nil {
				exitWithError("Couldn't generate article URL", joinError.Error())
			}
			newFeedItem.Link = &feeds.Link{Href: articleURL}
		}
	}

	rssFile := createFile(filepath.Join(outputFolder, "feed.xml"))
	rssData, rssError := feed.ToRss()
	if rssError != nil {
		exitWithError("Couldn't generate RSS feed", rssError.Error())
	}
	_, writeError := rssFile.WriteString(rssData)
	if writeError != nil {
		exitWithError("Couldn't write RSS feed", writeError.Error())
	}
}

// joinURLParts puts together two URL pieces without duplicating separators
// or removing separators. Before, this was done by path.Join directly which
// caused the resulting URL to be missing a forward slash behind the protocol.
// Using filepath.Join would cause incorrect separators on windows, as URLs
// should always use forward slashes, but windows uses backward slashes.
func joinURLParts(partOne, partTwo string) (string, error) {
	url, parseError := url.Parse(partOne)
	if parseError != nil {
		return "", parseError
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

func templateToString(temp *template.Template) string {
	buffer := &bytes.Buffer{}
	executionError := temp.Execute(buffer, nil)
	if executionError != nil {
		exitWithError(fmt.Sprintf("Couldn't execute template '%s'", temp.Name()), executionError.Error())
	}
	return buffer.String()
}

func templateToOptionalString(temp *template.Template) string {
	if temp == nil {
		return ""
	}

	return templateToString(temp)
}

type indexedArticle struct {
	pageConfig
	Title        string
	File         string
	RFC3339Time  time.Time
	podcastAudio string
	HumanTime    string
	Content      string
	Tags         []string
}

func timeFromRFC3339(value string) time.Time {
	time, timeParseError := time.Parse(time.RFC3339, value)
	if timeParseError != nil {
		exitWithError(fmt.Sprintf("Couldn't parse time '%s'. Format must match RFC3339", value), timeParseError.Error())
	}
	return time
}
