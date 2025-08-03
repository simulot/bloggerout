# Setup of the Blowfish theme for Hugo
I recommend using the Blowfish theme for Hugo sites. It is a modern, responsive theme that is easy to customize. You can find it on [Blowfish's page](https://blowfish.page/).

## Installing the Blowfish theme
The Git installation method remains simple and avoid the use of npm.

Here are the steps:
1. Navigate to your Hugo site's root directory.
```bash
cd /path/to/your/hugo/site
```
2. Run the following command to add the Blowfish theme as a submodule:
```bash
git init
git submodule add -b main https://github.com/nunocoracao/blowfish.git themes/blowfish
```
3. Copy theme's files from the `themes/blowfish/config/_default` directory to your site's `config/_default` directory:
```bash
mkdir -p config/_default
cp -r themes/blowfish/config/_default/* config/_default/
```


Other methods of installation are available on the [Blowfish documentation](https://blowfish.page/docs/installation/).


## Configuration

### Edit `config/_default/hugo.toml` 

Uncomment the line to enable the theme:
```toml
theme = "blowfish"
``` 

Change the base URL to match your site:
```toml
baseURL = "https://example.com/"
```


### Edit `config/_default/languages.en.toml`
Chan ge the blog name
```toml
title = "My Blog"
```
## Edit `config/_default/params.toml`

I have disabled image optimization. The optimization process does unnecessary image rotation with adjusting the image size. 

```toml
disableImageOptimization = true
disableImageOptimizationMD = true
```

Change showRecent to true to show recent posts in the home page. I have also set the number of recent posts to 5.
```toml
  showRecent = true
  showRecentItems = 5
  showMoreLink = true
```

To display the post's tag:
```toml
showTaxonomies = true
```

### Edit `config/_default/menu.en.toml`

To add  Post and Tags links to the main page, uncomment 
```toml
[[main]]
  name = "Blog"
  pageRef = "posts"
  weight = 10

[[main]]
  name = "Tags"
  pageRef = "tags"
  weight = 30

```

Add tags from the posts to the tags page.
```toml
[[tags]]
  name = "Tags"
  pageRef = "tags"
  weight = 20
```