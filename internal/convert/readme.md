# Convert Blogger to Hugo Markdown

This tool converts Blogger posts from a Blogger takeout to Hugo Markdown files. It processes the Blogger takeout directory, extracting posts and images, and organizes them into a structure suitable for Hugo.

It can uses a youtube takeout to get original video files and metadata, which are then linked in the posts.

It can also process Blogger takeouts from multiples co-authors, allowing you to merge posts from different authors into a single Hugo site including their original images.

## Features
- Converts Blogger posts to Hugo Markdown format.
- Use takeout's original photos when available.
- Uses the blogger image's when the original is not available in the takeout.
- Converts comments
- Supports Blogger takeouts from multiple authors.
- Supports Youtube takeouts to get original video files.
- Fast...


## Limitations
- It does not convert Blogger's layout, widgets, or themes.
- Many images and videos are not available in the Blogger takeout, so it will not be able to convert them. 
  - A `$conversion_error` tag will be added to the post if an image or video is not available in the takeout.
- Takeout's photos are stored into albums, with name possibly different from the blog title.
- Some posts have bizarre formatting.

## Posts conversion
Blogger posts are converted to Hugo Markdown files, using the [page bundle layout](https://gohugo.io/content-management/page-bundles/). Each post is stored in a directory named after the post's date and title. The content of the post is stored in a `index.md` file within that directory. Each related image is stored in the same directory with its original filename.

```
content/
├── 2021-01-01 My First Post/
│   ├── index.md
│   ├── IMG_20210101_123456.jpg
│   └── IMG_20210101_125256.jpg
└── 2021-01-02 My Second Post/
    ├── index.md
    └── IMG_20210102_123456.jpg
```

## Parameters

Convert uses the following parameters when generating posts. They use golang text/template syntax, allowing you to customize the output. 

### `--takeout`: The path to the Blogger takeout directory or zip files.

This parameter can be repeated to process multiples takeouts, like Blogger and Youtube, from several authors.

Example: `--takeout /home/user/Downloads/Author1/takeout-20250711T154429Z-*.zip --takeout /home/user/Downloads/Author2/takeout-*.zip --takeout /home/user/Downloads/youtube-takeout-*.zip`

All zip parts of the takeouts will be processed, extracting posts, images and videos from both Blogger and Youtube takeouts.

### `--hugo`: The path to the Hugo directory

It is required and must be specified when running the convert command. This is the root directory where the converted posts and images will be stored. It should point to your Hugo main directory.

It accepts a place holder for the blog name:
- `{{.Blog}}` 


Example: `/path/to/hugo/sites/{{.Blog}}` will set the hugo path with the blog name found in the blogger takeout.


### `--post-path`: The path template for posts inside the Hugo directory.

Default is `/content/posts/{{ .Title }}/`. 


This path is used to organize posts into directories based on their date. The template can include placeholders like:

 It can include placeholders like 
 - `{{ .Date.Format "2006-01-02" }}` to organize posts per date,
 - `{{ .Author }}` to organize by author

Example: `/content/posts/{{.Author}}/{{ .Date.Format "2006-01" }}` will create a directory structure like:  
```
content/
└── posts/
    ├── John Doe/
    │   └── 2021-01/
    │       ├── 2021-01-01 My First Post/
    │       │   ├── index.md
    │       │   └── IMG_20210101_125256.jpg
    │       └── My Second Post/
    │           ├── index.md
    │           └── IMG_20210102_123456.jpg
    └── Mary Smith/
        └── 2021-01/
            └── 2021-01-03Another Post/
                ├── index.md
                └── IMG_20210103_123456.jpg
```

> Note: <br>
> The time format uses the go language time format described in the go documentation [here](https://pkg.go.dev/time#pkg-constants) <br>

> Note: <br>
> The Date field can use any of go's time functions. 
> {{.Date.Year}}, {{.Date.Month}}, {{.Date.Day}}, {{.Date.Hour}}, {{.Date.Minute}}, {{.Date.Second}} are all available to use in the template.<br>
> Read the documentation [here](https://pkg.go.dev/time#Time) for more information.<br>



