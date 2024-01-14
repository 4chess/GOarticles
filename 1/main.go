package main

import (
    "encoding/json"
    "html/template"
    "io"
    "io/ioutil"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "strconv"
)

type Article struct {
    ID      int
    Title   string
    Message string
    File    string // Stores the file name
}

var (
    articles      []Article
    templatePath  = "templates"
    staticPath    = "static/articles"
    dataPath      = "data"
    articlesFile  = filepath.Join(dataPath, "articles.json")
    nextArticleID = 1
)

func main() {
    loadArticles()

    http.HandleFunc("/", showForm)
    http.HandleFunc("/submit", submitArticle)
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    http.ListenAndServe(":7070", nil)
}

func showForm(w http.ResponseWriter, r *http.Request) {
    tmpl, err := template.ParseFiles(filepath.Join(templatePath, "form.html"))
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, articles)
}

func submitArticle(w http.ResponseWriter, r *http.Request) {
    const maxUploadSize = 30 * 1024 * 1024 // 30 MB
    r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
    if err := r.ParseMultipartForm(maxUploadSize); err != nil {
        http.Error(w, "The uploaded file is too big. Please choose a file that's less than 30MB in size.", http.StatusBadRequest)
        return
    }

    title := r.FormValue("title")
    message := r.FormValue("message")
    if len(title) == 0 || len(title) > 75 || len(message) == 0 || len(message) > 200000 {
        http.Error(w, "Invalid title or message.", http.StatusBadRequest)
        return
    }

    file, handler, err := r.FormFile("file")
    fileName := ""
    if err == nil {
        defer file.Close()

        fileName = strconv.Itoa(nextArticleID) + filepath.Ext(handler.Filename)
        filePath := filepath.Join(staticPath, fileName)
        f, err := os.Create(filePath)
        if err != nil {
            http.Error(w, "Unable to create the file.", http.StatusInternalServerError)
            return
        }
        defer f.Close()
        _, err = io.Copy(f, file)
        if err != nil {
            http.Error(w, "Failed to save the file.", http.StatusInternalServerError)
            return
        }
    } else if err != http.ErrMissingFile {
        http.Error(w, "Invalid file.", http.StatusBadRequest)
        return
    }

    article := Article{
        ID:      nextArticleID,
        Title:   title,
        Message: message,
        File:    fileName,
    }
    articles = append(articles, article)
    saveArticleToFile(article)
    nextArticleID++

    http.Redirect(w, r, "/static/articles/"+strconv.Itoa(article.ID)+".html", http.StatusFound)
}

func loadArticles() {
    os.MkdirAll(dataPath, os.ModePerm)
    os.MkdirAll(staticPath, os.ModePerm)

    data, err := ioutil.ReadFile(articlesFile)
    if err != nil {
        if !os.IsNotExist(err) {
            panic(err)
        }
        return
    }

    if err := json.Unmarshal(data, &articles); err != nil {
        panic(err)
    }

    if len(articles) > 0 {
        nextArticleID = articles[len(articles)-1].ID + 1
    }
}

func saveArticles() {
    data, err := json.Marshal(articles)
    if err != nil {
        panic(err)
    }

    if err := ioutil.WriteFile(articlesFile, data, 0644); err != nil {
        panic(err)
    }
}

func saveArticleToFile(article Article) {
    funcMap := template.FuncMap{
        "endswith": strings.HasSuffix,
    }

    tmpl, err := template.New("article.html").Funcs(funcMap).ParseFiles(filepath.Join(templatePath, "article.html"))
    if err != nil {
        panic(err)
    }

    filePath := filepath.Join(staticPath, strconv.Itoa(article.ID)+".html")
    f, err := os.Create(filePath)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    if err := tmpl.Execute(f, article); err != nil {
        panic(err)
    }

    saveArticles()
}


