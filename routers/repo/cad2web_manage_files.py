#!/usr/bin/env python
# coding: utf-8

r"""Files paths and manipulation procedures of Gitea for CAD.

Called from Go code. The parameters are passed as command line arguments
in the Go code.

"""

from __future__ import print_function, absolute_import

import logging
import hashlib
from os.path import join

from requests import get

logger = logging.getLogger(__name__)


def _download_file(url, filename):
    r"""Download an external file (at specified url) to a local file

    Parameters
    ----------
    url : str
        URL to the file to be downloaded
    filename : str
        Full path to the local file that is to be created by the download

    Raises
    ------
    requests.exceptions.HTTPError if the URL points to a file
    that does not exist
    FileNotFoundError if the filename cannot be opened

    """
    logger.info("Downloading file at URL : %s" % url)
    response = get(url, stream=True)
    logger.info("Response is %s" % str(response))
    response.raise_for_status()

    with open(filename, 'wb') as f:
        for chunk in response.iter_content(1024):
            f.write(chunk)


def sha1(filename):
    r"""Compute the SHA-1 hash of a file

    Parameters
    ----------
    filename : str
        Full path to file

    Notes
    -----
    The typical call to this function takes 2-3 ms

    Returns
    -------
    str : The SHA-1 has of the file

    Raises
    ------
    FileNotFoundError if filename points to a file that does not exist

    """
    logger.debug("Computing SHA-1 hash of : %s" % filename)
    sha1sum = hashlib.sha1()
    with open(filename, 'rb') as source:
        block = source.read(2 ** 16)
        while len(block) != 0:
            sha1sum.update(block)
            block = source.read(2 ** 16)
    sha1_hash = sha1sum.hexdigest()
    logger.debug("SHA-1 hash is : %s" % sha1_hash)
    return sha1_hash


def _conversion_filename(file_in, folder_out, i=0):
    r"""Build the name of the converted file using the name of the file to be
    converted

    Parameters
    ----------
    file_in : str
        Path to the input_file
    folder_out : str
        Path to the output folder
    i : int
        Index, as a file may lead to the creation of many converted files
        if it contains multiple shapes

    Returns
    -------
    str : Path to the converted file

    """
    logger.debug("Call to _conversion_filename()")
    # return join(folder, "%s_%i.stl" % (name, i))

    hash_name = sha1(file_in)
    logger.debug("End of call to _conversion_filename()")
    return join(folder_out, "%s_%i.json" % (hash_name, i))


def _descriptor_filename(converted_files_folder, cad_file_basename):
    r"""Build the name of the file that contains the results of the
    conversion process

    Parameters
    ----------
    converted_files_folder : str
        Path to the folder where the converted files end up
    cad_file_basename : str
        Base name of the CAD file that is being converted

    Returns
    -------
    str : full path to the descriptor file

    """
    return join(converted_files_folder, "%s.%s" % (cad_file_basename, "dat"))


def _write_descriptor(max_dim, names, descriptor_filename_):
    r"""Write the contents of a descriptor file

    Parameters
    ----------
    max_dim : float
        Maximum dimension of the bounding box of all objects in the CAD file
    names : list[str]
        Names of all files that should be loaded by three.js to build the
        web display of the CAD file

    Returns
    -------
    Nothing, it is a procedure

    """
    with open(descriptor_filename_, 'w') as f:
        f.write("%f\n" % max_dim)
        f.write("\n".join(names))
