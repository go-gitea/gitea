// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "text/x-dockerfile");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT("simple_nodejs_dockerfile",
     "[keyword FROM] node:carbon",
     "[comment # Create app directory]",
     "[keyword WORKDIR] /usr/src/app",
     "[comment # Install app dependencies]",
     "[comment # A wildcard is used to ensure both package.json AND package-lock.json are copied]",
     "[comment # where available (npm@5+)]",
     "[keyword COPY] package*.json ./",
     "[keyword RUN] npm install",
     "[keyword COPY] . .",
     "[keyword EXPOSE] [number 8080] [number 3000]",
     "[keyword ENV] NODE_ENV development",
     "[keyword CMD] [[ [string \"npm\"], [string \"start\"] ]]");

  // Ideally the last space should not be highlighted.
  MT("instruction_without_args_1",
     "[keyword CMD] ");

  MT("instruction_without_args_2",
     "[comment # An instruction without args...]",
     "[keyword ARG] [error #...is an error]");

  MT("multiline",
     "[keyword RUN] apt-get update && apt-get install -y \\",
     "  mercurial \\",
     "  subversion \\",
     "  && apt-get clean \\",
     "  && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*");

  MT("from_comment",
     "  [keyword FROM] debian:stretch # I tend to use stable as that is more stable",
     "  [keyword FROM] debian:stretch [keyword AS] stable # I am even more stable",
     " [keyword FROM] [error # this is an error]");

  MT("from_as",
     "[keyword FROM] golang:1.9.2-alpine3.6 [keyword AS] build",
     "[keyword COPY] --from=build /bin/project /bin/project",
     "[keyword ENTRYPOINT] [[ [string \"/bin/project\"] ]]",
     "[keyword CMD] [[ [string \"--help\"] ]]");

  MT("arg",
     "[keyword ARG] VERSION=latest",
     "[keyword FROM] busybox:$VERSION",
     "[keyword ARG] VERSION",
     "[keyword RUN] echo $VERSION > image_version");

  MT("label",
     "[keyword LABEL] com.example.label-with-value=[string \"foo\"]");

  MT("label_multiline",
     "[keyword LABEL] description=[string \"This text illustrates ]\\",
     "[string that label-values can span multiple lines.\"]");

  MT("maintainer",
     "[keyword MAINTAINER] Foo Bar [string \"foo@bar.com\"] ",
     "[keyword MAINTAINER] Bar Baz <bar@baz.com>");

  MT("env",
     "[keyword ENV] BUNDLE_PATH=[string \"$GEM_HOME\"] \\",
     "  BUNDLE_APP_CONFIG=[string \"$GEM_HOME\"]");

  MT("verify_keyword",
     "[keyword RUN] add-apt-repository ppa:chris-lea/node.js");

  MT("scripts",
     "[comment # Set an entrypoint, to automatically install node modules]",
     "[keyword ENTRYPOINT] [[ [string \"/bin/bash\"], [string \"-c\"], [string \"if [[ ! -d node_modules ]]; then npm install; fi; exec \\\"${@:0}\\\";\"] ]]",
     "[keyword CMD] npm start",
     "[keyword RUN] npm run build && \\",
     "[comment # a comment between the shell commands]",
     "  npm run test");

  MT("strings_single",
     "[keyword FROM] buildpack-deps:stretch",
     "[keyword RUN] { \\",
     "        echo [string 'install: --no-document']; \\",
     "        echo [string 'update: --no-document']; \\",
     "    } >> /usr/local/etc/gemrc");

  MT("strings_single_multiline",
     "[keyword RUN] set -ex \\",
     "    \\",
     "    && buildDeps=[string ' ]\\",
     "[string        bison ]\\",
     "[string        dpkg-dev ]\\",
     "[string        libgdbm-dev ]\\",
     "[string        ruby ]\\",
     "[string    '] \\",
     "    && apt-get update");

  MT("strings_single_multiline_2",
     "[keyword RUN] echo [string 'say \\' ]\\",
     "[string   it works'] ");

  MT("strings_double",
     "[keyword RUN] apt-get install -y --no-install-recommends $buildDeps \\",
     " \\",
     " && wget -O ruby.tar.xz [string \"https://cache.ruby-lang.org/pub/ruby/${RUBY_MAJOR%-rc}/ruby-$RUBY_VERSION.tar.xz\"] \\",
     " && echo [string \"$RUBY_DOWNLOAD_SHA256 *ruby.tar.xz\"] | sha256sum -c - ");

  MT("strings_double_multiline",
     "[keyword RUN] echo [string \"say \\\" ]\\",
     "[string   it works\"] ");

  MT("escape",
     "[comment # escape=`]",
     "[keyword FROM] microsoft/windowsservercore",
     "[keyword RUN] powershell.exe -Command `",
     "    $ErrorActionPreference = [string 'Stop']; `",
     "    wget https://www.python.org/ftp/python/3.5.1/python-3.5.1.exe -OutFile c:\python-3.5.1.exe ; `",
     "    Start-Process c:\python-3.5.1.exe -ArgumentList [string '/quiet InstallAllUsers=1 PrependPath=1'] -Wait ; `",
     "    Remove-Item c:\python-3.5.1.exe -Force)");

  MT("escape_strings",
     "[comment # escape=`]",
     "[keyword FROM] python:3.6-windowsservercore [keyword AS] python",
     "[keyword RUN] $env:PATH = [string 'C:\\Python;C:\\Python\\Scripts;{0}'] -f $env:PATH ; `",
     // It should not consider \' as escaped.
     // "  Set-ItemProperty -Path [string 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment\\'] -Name Path -Value $env:PATH ;");
     "  Set-ItemProperty -Path [string 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment\\' -Name Path -Value $env:PATH ;]");
})();
