#!/opt/miniconda3/bin/python3

import sys
from os.path import splitext, basename

from requests import get

from aocxchange.step import StepImporter
from aocxchange.stl import StlExporter

GITEA_URL = "http://localhost:3000"
# GITEA_URL = "http://127.0.0.1:3000"


def download_file(url, filename):
    r"""Download an external file (at specified url) to a local file

    Parameters
    ----------
    url : str
        URL to the file to be downloaded
    filename : str
        Full path to the local file

    """
    print("url in download function is : %s" % url)
    response = get(url, stream=True)

    print("response is %s" % str(response))

    response.raise_for_status()

    with open(filename, 'wb') as f:
        for chunk in response.iter_content(1024):
            f.write(chunk)


def main():
    r"""Procedure that handles the conversion from a CAD file to a format usable by a 3D web viewer

    The parameters are command line arguments retrieved in this procedure

    """
    cad_file_raw_url = sys.argv[1]
    cad_file_raw_url_full = "%s%s" % (GITEA_URL, cad_file_raw_url)

    cad_file_extension = splitext(cad_file_raw_url)[1]
    cad_file_basename = basename(cad_file_raw_url)

    converted_files_folder = sys.argv[2]

    # - DEBUG
    print("CAD Converter with params : %s & %s" % (cad_file_raw_url, converted_files_folder))
    print("cad_file_extension is : %s" % cad_file_extension)
    print("cad_file_basename is : %s" % cad_file_basename)
    print("cad_file_raw_url_full is : %s" % cad_file_raw_url_full)

    cad_file_filename = "%s/%s" % (converted_files_folder, cad_file_basename)
    converted_files_descriptor_filename = "%s/%s.%s" % (converted_files_folder, cad_file_basename, "dat")

    download_file(cad_file_raw_url_full, cad_file_filename)

    print("cad_file_filename is : %s" % cad_file_filename)

    if cad_file_extension.lower() in [".fcstd"]:
        pass

    elif cad_file_extension.lower() in [".step", ".stp"]:
        converted_filenames = []
        shapes = StepImporter(cad_file_filename).shapes
        for i, shape in enumerate(shapes):
            converted_filename = "%s/%s_%i.stl" % (converted_files_folder, cad_file_basename, i)
            e = StlExporter(filename=converted_filename, ascii_mode=True)
            e.set_shape(shape)
            e.write_file()
            converted_filenames.append(basename(converted_filename))

        with open(converted_files_descriptor_filename, 'w') as f:
            f.write("\n".join(converted_filenames))

    elif cad_file_extension.lower() in [".iges", ".igs"]:
        pass
    elif cad_file_extension.lower() in [".brep", ".brp"]:
        pass
    elif cad_file_extension.lower in [".stl"]:
        pass
    elif cad_file_extension.lower() in [".py"]:
        pass
    else:
        raise ValueError("Unknown CAD cad_file_extension")

    sys.exit(0)


main()
