#!/opt/miniconda3/bin/python3

import logging
import sys
import zipfile
from os.path import splitext, basename

from requests import get

from aocxchange.step import StepImporter
from aocxchange.iges import IgesImporter
from aocxchange.brep import BrepImporter
from aocxchange.stl import StlExporter

GITEA_URL = "http://localhost:3000"
# GITEA_URL = "http://127.0.0.1:3000"

logger = logging.getLogger(__name__)


def download_file(url, filename):
    r"""Download an external file (at specified url) to a local file

    Parameters
    ----------
    url : str
        URL to the file to be downloaded
    filename : str
        Full path to the local file

    """
    logger.info("Downloading file at URL : %s" % url)
    response = get(url, stream=True)
    logger.info("Response is %s" % str(response))
    response.raise_for_status()

    with open(filename, 'wb') as f:
        for chunk in response.iter_content(1024):
            f.write(chunk)


def main():
    r"""Procedure that handles the conversion from a CAD file to a format usable by a 3D web viewer

    The parameters are command line arguments retrieved in this procedure

    """

    logging.basicConfig(level=logging.DEBUG,
                        format='%(asctime)s :: %(levelname)8s :: %(module)20s '
                               ':: %(lineno)3d :: %(message)s')

    cad_file_raw_url = sys.argv[1]
    cad_file_raw_url_full = "%s%s" % (GITEA_URL, cad_file_raw_url)

    cad_file_extension = splitext(cad_file_raw_url)[1]
    cad_file_basename = basename(cad_file_raw_url)

    converted_files_folder = sys.argv[2]

    logger.info("Params for view_cad_converter.py main(): %s & %s" % (cad_file_raw_url, converted_files_folder))
    logger.info("cad_file_extension is : %s" % cad_file_extension)
    logger.info("cad_file_basename is : %s" % cad_file_basename)
    logger.info("cad_file_raw_url_full is : %s" % cad_file_raw_url_full)

    cad_file_filename = "%s/%s" % (converted_files_folder, cad_file_basename)
    converted_files_descriptor_filename = "%s/%s.%s" % (converted_files_folder, cad_file_basename, "dat")

    download_file(cad_file_raw_url_full, cad_file_filename)

    logger.info("cad_file_filename is : %s" % cad_file_filename)

    if cad_file_extension.lower() in [".fcstd"]:
        converted_filenames = []
        logger.info("Starting FreeCAD conversion")

        # unzip and extract the breps
        fcstd_as_zip = zipfile.ZipFile(cad_file_filename)
        fcstd_contents = fcstd_as_zip.namelist()
        logger.info("fcstd_contents is : %s" % str(fcstd_contents))
        for i, name in enumerate(fcstd_contents):
            # convert the breps to stl
            if splitext(name)[1].lower() in [".brep", ".brp"]:
                fcstd_as_zip.extract(name, converted_files_folder)
                logger.info("input to BRep importer is : %s" % ("%s/%s" % (converted_files_folder, name)))
                shape = BrepImporter("%s/%s" % (converted_files_folder, name)).shape
                converted_filename = "%s/%s_%i.stl" % (converted_files_folder, name, i)
                converted_filenames.append(basename(converted_filename))
                try:
                    e = StlExporter(filename=converted_filename, ascii_mode=True)
                    e.set_shape(shape)
                    e.write_file()

                    # build the descriptor
                    with open(converted_files_descriptor_filename, 'w') as f:
                        f.write("\n".join(converted_filenames))
                except RuntimeError:
                    logger.error("RuntimeError for %s" % name)

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
        converted_filenames = []
        shapes = IgesImporter(cad_file_filename).shapes
        for i, shape in enumerate(shapes):
            converted_filename = "%s/%s_%i.stl" % (converted_files_folder, cad_file_basename, i)
            e = StlExporter(filename=converted_filename, ascii_mode=True)
            e.set_shape(shape)
            e.write_file()
            converted_filenames.append(basename(converted_filename))

        with open(converted_files_descriptor_filename, 'w') as f:
            f.write("\n".join(converted_filenames))

    elif cad_file_extension.lower() in [".brep", ".brp"]:

        shape = BrepImporter(cad_file_filename).shape
        converted_filename = "%s/%s_%i.stl" % (converted_files_folder, cad_file_basename, 0)
        converted_filenames = [basename(converted_filename)]
        e = StlExporter(filename=converted_filename, ascii_mode=True)
        e.set_shape(shape)
        e.write_file()

        with open(converted_files_descriptor_filename, 'w') as f:
            f.write("\n".join(converted_filenames))

    elif cad_file_extension.lower() in [".stl"]:
        converted_filenames = [cad_file_basename]
        with open(converted_files_descriptor_filename, 'w') as f:
            f.write("\n".join(converted_filenames))

    elif cad_file_extension.lower() in [".py"]:
        pass

    else:
        msg = "Unknown CAD cad_file_extension : %s" % cad_file_extension
        logger.error(msg)
        raise ValueError(masg)

    sys.exit(0)


main()
