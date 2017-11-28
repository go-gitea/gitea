FROM webhippie/golang:edge
RUN apk add --update autoconf zlib-dev > /dev/null && \
    mkdir build && \
    curl -sL "https://github.com/git/git/archive/v1.7.2.tar.gz" -o git.tar.gz && \
    tar -C build -xzf git.tar.gz && \
    cd build/git-1.7.2 && \
    { autoconf 2> err || { cat err && false; } } && \
    ./configure --without-tcltk --prefix=/opt/git-1.7.2 > /dev/null && \
    { make install NO_PERL=please > /dev/null 2> err || { cat err && false; } } && \
    cd ../.. && \
    rm -rf build git.tar.gz \
