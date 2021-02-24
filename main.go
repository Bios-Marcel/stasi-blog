package main

import (
	"bytes"
	"embed"
	"encoding/json"
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

const outputCustomPages string = "pages"
const outputArticles string = "articles"

var (
	//go:embed skeletons/*
	skeletonFS embed.FS

	writtenFiles []string
)

func main() {
	var configPath string
	if config != nil && *config != "" {
		configPath = *config
	} else {
		configPath = filepath.Join(*input, "config.json")
	}
	configFile, openError := os.Open(configPath)
	if openError != nil {
		log.Fatalf("Error loading config: %s\n", openError)
	}

	//Delete old data
	os.RemoveAll(filepath.Join(*output, "media"))
	os.RemoveAll(filepath.Join(*output, "articles"))
	os.RemoveAll(filepath.Join(*output, "pages"))
	os.Remove(filepath.Join(*output, "favicon.ico"))
	os.Remove(filepath.Join(*output, "base.css"))
	os.Remove(filepath.Join(*output, "404.html"))
	os.Remove(filepath.Join(*output, "feed.xml"))
	files, globError := filepath.Glob(filepath.Join(*output, "index*.html"))
	if globError != nil {
		panic(globError)
	}
	for _, indexToDelete := range files {
		os.Remove(indexToDelete)
	}

	//Create empty directories
	createDirectory(filepath.Join(*output, "media"))
	createDirectory(filepath.Join(*output, "articles"))
	createDirectory(filepath.Join(*output, "pages"))

	loadedPageConfig := pageConfig{
		DateFormat: "2 January 2006",
		UseFavicon: true,
	}
	configDecodeError := json.NewDecoder(configFile).Decode(&loadedPageConfig)
	if configDecodeError != nil {
		log.Fatalf("Error decoding config: %s\n", configDecodeError)
	}

	if loadedPageConfig.UseFavicon {
		faviconSourcePath := filepath.Join(*input, "favicon.ico")
		_, faviconErr := os.Stat(faviconSourcePath)
		if faviconErr != nil {
			log.Fatalln("favicon.ico couldn't be found. If you don't want to use a favicon, set 'UseFavicon' to 'false'.")
		} else {
			copyFileByPath(faviconSourcePath, filepath.Join(*output, "favicon.ico"))
		}
	}

	baseTemplate, parseError := template.ParseFS(skeletonFS, "skeletons/base.html")
	if parseError != nil {
		panic(parseError)
	}

	customPageFiles, readError := ioutil.ReadDir(filepath.Join(*input, "pages"))
	if readError != nil {
		panic(readError)
	}

	customPageSkeleton, parseError := template.ParseFS(skeletonFS, "skeletons/page.html")
	if parseError != nil {
		panic(parseError)
	}
	addSubTemplates(customPageSkeleton, baseTemplate)

	customPageTemplates := make(map[string]*template.Template)
	var customPages []*customPageEntry
	for _, customPage := range customPageFiles {
		customPageSkeletonClone, cloneError := customPageSkeleton.Clone()
		if cloneError != nil {
			panic(cloneError)
		}

		sourcePath := filepath.Join(*input, "pages", customPage.Name())
		customPageTemplate, parseError := customPageSkeletonClone.ParseFiles(sourcePath)
		if parseError != nil {
			panic(parseError)
		}

		newCustomPageFileName := filepath.Join(outputCustomPages, customPage.Name())
		customPageTemplates[newCustomPageFileName] = customPageTemplate

		customPages = append(customPages, &customPageEntry{
			Title: templateToString(customPageTemplate.Lookup("title")),
			File:  newCustomPageFileName,
		})
	}

	articleSkeleton, parseError := template.ParseFS(skeletonFS, "skeletons/article.html")
	if parseError != nil {
		panic(parseError)
	}

	addSubTemplates(articleSkeleton, baseTemplate)

	articles, readError := ioutil.ReadDir(filepath.Join(*input, "articles"))
	if readError != nil {
		panic(readError)
	}

	var indexedArticles []*indexedArticle
	for _, article := range articles {
		//Other files are ignored. For example I use this to create
		//.html-draft files which are posts that I don't want to publish
		//yet, but still have in the blog source directory.
		if !strings.HasSuffix(article.Name(), ".html") {
			continue
		}

		newArticleSkeleton, cloneError := articleSkeleton.Clone()
		if cloneError != nil {
			panic(cloneError)
		}
		sourcePath := filepath.Join(*input, "articles", article.Name())
		specificArticleTemplate, parseError := newArticleSkeleton.ParseFiles(sourcePath)
		if parseError != nil {
			panic(parseError)
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
		publishTime, timeParseError := time.Parse("2006-01-02", templateToString(specificArticleTemplate.Lookup("date")))
		if timeParseError != nil {
			panic(timeParseError)
		}
		articleData.RFC3339Time = publishTime.Format(time.RFC3339)
		articleData.HumanTime = publishTime.Format(loadedPageConfig.DateFormat)
		articleData.PodcastAudio = templateToOptionalString(specificArticleTemplate.Lookup("podcast-audio"))
		if articleData.PodcastAudio != "" {
			if strings.HasPrefix(strings.TrimPrefix(articleData.PodcastAudio, "/"), "media") {
				articleData.PodcastAudio = path.Join(loadedPageConfig.BasePath, articleData.PodcastAudio)
			}
		}
		articleTargetPath := filepath.Join(outputArticles, article.Name())

		writeTemplateToFile(specificArticleTemplate, articleData, articleTargetPath)

		newIndexedArticle := &indexedArticle{
			pageConfig:   loadedPageConfig,
			podcastAudio: templateToOptionalString(specificArticleTemplate.Lookup("podcast-audio")),
			Title:        templateToString(specificArticleTemplate.Lookup("title")),
			File:         articleTargetPath,
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

	//We first populate the custom page array so that the pages all have a correct menu header.
	for fileName, customPageTemplate := range customPageTemplates {
		writeTemplateToFile(customPageTemplate, &customPageData{
			pageConfig:  loadedPageConfig,
			CustomPages: customPages,
		}, fileName)
	}

	indexSkeleton, parseError := template.ParseFS(skeletonFS, "skeletons/index.html")
	if parseError != nil {
		panic(parseError)
	}
	addSubTemplates(indexSkeleton, baseTemplate)

	//Main Index with all articles.
	writeTemplateToFile(indexSkeleton, &indexData{
		pageConfig:      loadedPageConfig,
		Tags:            tags,
		CustomPages:     customPages,
		IndexedArticles: indexedArticles,
	}, "index.html")

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

		writeTemplateToFile(indexSkeleton, &indexData{
			pageConfig:      loadedPageConfig,
			Tags:            tags,
			FilterTag:       tag,
			CustomPages:     customPages,
			IndexedArticles: tagFilteredArticles,
		}, "index-tag-"+tag+".html")
	}

	writeRSSFeed(indexedArticles, loadedPageConfig)

	baseCSSFile, fsError := skeletonFS.Open("skeletons/base.css")
	if fsError != nil {
		panic(fsError)
	}

	if *minifyOutput {
		baseCSSOutput := createFile(filepath.Join(*output, "base.css"))
		minifyError := minifier.Minify("text/css", baseCSSOutput, baseCSSFile)
		if minifyError != nil {
			panic(minifyError)
		}
	} else {
		copyDataIntoFile(baseCSSFile, filepath.Join(*output, "base.css"))
	}

	copy.Copy(filepath.Join(*input, "media"), filepath.Join(*output, "media"))

	notFoundSkeleton, parseError := template.ParseFS(skeletonFS, "skeletons/404.html")
	if parseError != nil {
		panic(parseError)
	}
	addSubTemplates(notFoundSkeleton, baseTemplate)
	writeTemplateToFile(notFoundSkeleton, &customPageData{
		pageConfig:  loadedPageConfig,
		CustomPages: customPages,
	}, "404.html")
}

func writeRSSFeed(articles []*indexedArticle, loadedPageConfig pageConfig) {
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
			audioFile, statError := os.Stat(filepath.Join(*input, article.podcastAudio))
			if statError != nil {
				panic(statError)
			}
			newFeedItem.Enclosure = &feeds.Enclosure{
				Type:   "audio/mp3",
				Length: strconv.FormatInt(audioFile.Size(), 10),
				Url:    joinURLParts(feed.Link.Href, article.podcastAudio),
			}
		}
		feed.Items = append(feed.Items, newFeedItem)
		if loadedPageConfig.URL != "" {
			newFeedItem.Link = &feeds.Link{Href: joinURLParts(loadedPageConfig.URL, article.File)}
		}
	}

	rssFile := createFile(filepath.Join(*output, "feed.xml"))
	rssData, rssError := feed.ToRss()
	if rssError != nil {
		panic(rssData)
	}
	_, writeError := rssFile.WriteString(rssData)
	if writeError != nil {
		panic(writeError)
	}
}

// joinURLParts puts together two URL pieces without duplicating separators
// or removing separators. Before, this was done by path.Join directly which
// caused the resulting URL to be missing a forward slash behind the protocol.
// Using filepath.Join would cause incorrect separators on windows, as URLs
// should always use forward slashes, but windows uses backward slashes.
func joinURLParts(partOne, partTwo string) string {
	url, parseError := url.Parse(partOne)
	if parseError != nil {
		panic(parseError)
	}

	url.Path = path.Join(url.Path, partTwo)
	return url.String()
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
}

func addSubTemplates(targetTemplate *template.Template, sourceTemplates *template.Template) {
	for _, subTemplate := range sourceTemplates.Templates() {
		_, addError := targetTemplate.AddParseTree(subTemplate.Name(), subTemplate.Tree)
		if addError != nil {
			panic(addError)
		}
	}
}

func templateToString(temp *template.Template) string {
	buffer := &bytes.Buffer{}
	executionError := temp.Execute(buffer, nil)
	if executionError != nil {
		panic(executionError)
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
		panic(timeParseError)
	}
	return time
}
