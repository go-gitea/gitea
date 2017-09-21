gorst
=====

.. image:: https://travis-ci.org/hhatto/gorst.svg?branch=master
    :target: https://travis-ci.org/hhatto/gorst
    :alt: Build status

.. image:: https://godoc.org/github.com/hhatto/gorst?status.png
    :target: http://godoc.org/github.com/hhatto/gorst
    :alt: GoDoc

This is a Go_ implementation of reStructuredText_. developed on the basis of `Go markdown module implemented by Michael Teichgräber`_ .

Only Support for HTML output is implemented.

.. _reStructuredText: http://docutils.sourceforge.net/docs/ref/rst/restructuredtext.html
.. _Go: http://golang.org/
.. _`Go markdown module implemented by Michael Teichgräber`: https://github.com/knieriem/markdown

**This is experimental module. Highly under development.**


Installation
------------
.. code-block:: bash

    $ go get github.com/hhatto/gorst


Usage
-----
.. code-block:: go

    package main

    import (
        "bufio"
        "os"
        "github.com/hhatto/gorst"
    )

    func main() {
        p := rst.NewParser(nil)

        w := bufio.NewWriter(os.Stdout)
        p.ReStructuredText(os.Stdin, rst.ToHTML(w))
        w.Flush()
    }


TODO
----
* Simple Table
* Footnotes
* Citations
* Directives (figure, contents, ...)
* etc...
