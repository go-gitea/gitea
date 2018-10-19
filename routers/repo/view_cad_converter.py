#!/opt?miniconda3/bin/python3

import sys
from os.path import splitext, basename
import httplib2

from aocxchange.step import StepImporter
from aocxchange.stl import StlExporter

GITEA_URL = "http://localhost:3000"
# GITEA_URL = "http://127.0.0.1:3000"


def download_file(url, filename):
    # h = httplib2.Http(".cache")
    # resp, content = h.request(url, "GET")
    # if isinstance(content, bytes):
    #     with open(filename, 'wb') as f:
    #         f.write(content)
    # else:
    #     with open(filename, 'w') as f:
    #         f.write(content)

    from requests import get  # to make GET request

    headers = {
        'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.95 Safari/537.36'
    }

    # open in binary mode
    print("url in download function is : %s" % url)
    response = get(url, stream=True, headers=headers)
    print("response is %s" % str(response))


    response.raise_for_status()
    with open(filename, 'wb') as playFile:
        for chunk in res.iter_content(1024):
            playFile.write(chunk)

    # import urllib.request
    #
    # # TODO : solve the port problem
    #
    # with urllib.request.urlopen(fileurl) as url:
    #     s = url.read()
    # print("s is %s" % s)

    #
    #
    # from subprocess import call
    # call(["wget", fileurl, "--directory-prefix=%s" % target_folder])


def main():

    url_raw_file = sys.argv[1]
    url_raw_file_full = "%s%s" % (GITEA_URL, url_raw_file)

    extension = splitext(url_raw_file)[1]
    basename_ = basename(url_raw_file)

    target_folder = sys.argv[2]

    print("Hello from CAD Converter with params : %s & %s" % (url_raw_file, target_folder))
    print("extension is : %s" % extension)
    print("basename is : %s" % basename_)
    print("url_raw_file_full is : %s" % url_raw_file_full)

    filename = "%s/%s" % (target_folder, basename_)

    download_file(url_raw_file_full, filename)

    print("filename is : %s" % filename)

    if extension.lower() in [".fcstd"]:
        pass
    elif extension.lower() in [".step", ".stp"]:
        names = []
        shapes = StepImporter(filename).shapes
        for i, shape in enumerate(shapes):
            name = "%s/%s_%i.stl" % (target_folder, basename_, i)
            e = StlExporter(filename=name, ascii_mode=True)
            e.set_shape(shape)
            e.write_file()
            names.append(name)
        sys.exit(",".join(names))
    elif extension.lower() in [".iges", ".igs"]:
        pass
    elif extension.lower() in [".brep", ".brp"]:
        pass
    elif extension.lower in [".stl"]:
        pass
    elif extension.lower() in [".py"]:
        pass
    else:
        raise ValueError("Unknown CAD extension")

    sys.exit("toto")


main()
