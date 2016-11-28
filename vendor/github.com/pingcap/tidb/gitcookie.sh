touch ~/.gitcookies
chmod 0600 ~/.gitcookies

git config --global http.cookiefile ~/.gitcookies

tr , \\t <<\__END__ >>~/.gitcookies
go.googlesource.com,FALSE,/,TRUE,2147483647,o,git-z.pingcap.com=1/Xv6CBlnVpdrhYBXT5i_VexGocQcbgkKsrW938zgjqx0
go-review.googlesource.com,FALSE,/,TRUE,2147483647,o,git-z.pingcap.com=1/Xv6CBlnVpdrhYBXT5i_VexGocQcbgkKsrW938zgjqx0
__END__
