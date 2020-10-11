#!/usr/bin/env python
# coding: utf-8

r"""Conversion procedure of Gitea for CAD.

Called from Go code. The parameters are passed as command line arguments
in the Go code.

"""

from __future__ import print_function, absolute_import

import logging
from os.path import splitext, basename, join
import sys
import time

from cad2web_manage_files import _download_file
from cad2web_freecad import convert_freecad_file
from cad2web_neutral_formats import convert_step_file, convert_iges_file, \
    convert_brep_file, convert_stl_file
from cad2web_py import convert_py_file
from cad2web_stepzip import convert_stepzip_file


# Works in any case : local dev and server
GITEA_URL = "http://localhost:3000"
# GITEA_URL = "http://127.0.0.1:3000"

logger = logging.getLogger(__name__)


def main():
    r"""Procedure that handles the conversion from a CAD file to a format
    usable by a 3D web viewer

    The parameters are command line arguments retrieved in this procedure

    """

    t0 = time.time()
    logging.basicConfig(level=logging.DEBUG,
                        format='%(asctime)s :: %(levelname)8s :: %(module)20s '
                               ':: %(lineno)3d :: %(message)s')

    # Retrieve parameters from command
    # cad_file_raw_url = sys.argv[1]  # Direct download URL for the file
    # raw_link = sys.argv[1]
    # tree_path = sys.argv[2]
    # cad_file_raw_url = "%s/%s" % (raw_link, tree_path)
    cad_file_raw_url = sys.argv[1]
    converted_files_folder = sys.argv[2]  # Root destination for converted files

    logger.debug("sys.argv[1]       (CAD file raw url) = %s" % cad_file_raw_url)
    logger.debug("sys.argv[2] (Converted files folder) = %s" % converted_files_folder)

    branch = cad_file_raw_url.split("/")[5]
    user = cad_file_raw_url.split("/")[1]
    project = cad_file_raw_url.split("/")[2]

    clone_url = "%s/%s/%s" % (GITEA_URL, user, project)

    logger.info("Branch is %s" % branch)
    logger.info("User is %s" % user)
    logger.info("Project is %s" % project)

    cad_file_raw_url_full = "%s%s" % (GITEA_URL, cad_file_raw_url)

    cad_file_filename = join(converted_files_folder, basename(cad_file_raw_url))

    # Download the original CAD file to the converted files folder
    # (it will be deleted after conversion)
    _download_file(cad_file_raw_url_full, cad_file_filename)

    cad_file_extension = splitext(cad_file_raw_url)[1]

    conversion_function = {".fcstd": convert_freecad_file,
                           ".step": convert_step_file,
                           ".stp": convert_step_file,
                           ".iges": convert_iges_file,
                           ".igs": convert_iges_file,
                           ".brep": convert_brep_file,
                           ".brp": convert_brep_file,
                           ".stl": convert_stl_file,
                           ".py": convert_py_file,
                           ".stepzip": convert_stepzip_file}

    try:
        if splitext(cad_file_raw_url)[1].lower() != ".py":
            logger.info("Dealing with a static CAD file")
            conversion_function[splitext(cad_file_raw_url)[1].lower()](
                cad_file_filename, converted_files_folder)
        else:
            logger.info("Dealing with a scripted CAD file")
            convert_py_file(cad_file_filename,
                            converted_files_folder,
                            clone_url,
                            branch)
        t1 = time.time()
        logger.info("The whole Python call took %f" % (t1 - t0))
        sys.exit(0)
    except KeyError:
        msg = "Unknown CAD cad_file_extension : %s" % cad_file_extension
        logger.error(msg)
        t1 = time.time()
        logger.info("The whole Python call took %f" % (t1 - t0))
        raise ValueError(msg)


if __name__ == "__main__":
    main()
