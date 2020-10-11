#!/usr/bin/env python
# coding: utf-8

r"""OCC shape conversion to JSON"""

from __future__ import print_function, absolute_import

import json
import logging
from os.path import isfile
import uuid

from OCC.Core.Visualization import Tesselator

# from aocxchange.stl import StlExporter

# we use our own version of TopologyUtils.py as some functions were not
# available in the OCC that was installed
# from OCC.Extend.TopologyUtils import is_edge, is_wire, discretize_edge, \
#     discretize_wire
from cad2web_topology import is_edge, is_wire, discretize_edge, \
    discretize_wire, TopologyExplorer


logger = logging.getLogger(__name__)


def _convert_shape(shape, filename):
    r"""Convert a shape to a format usable for web display.

    The currently used format for web display is JSON

    Parameters
    ----------
    shape : OCC shape
    filename : full path to the target file name

    Returns
    -------
    Nothing, it is a procedure, not a function

    """
    logger.debug("Call to _convert_shape()")
    if isfile(filename):
        logger.info("Using existing file")
        pass  # The cached version will be used
    else:
        logger.info("Converting shape")
        _shape_to_json(shape, filename)
    logger.debug("End of call to _convert_shape()")


def color_to_hex(rgb_color):
    """ Takes a tuple with 3 floats between 0 and 1. Useful to convert occ
    colors to web color code

    Parameters
    ----------
    rgb_color : tuple of floats between 0. and 1.

    Returns
    -------
    Returns a hex.

    """
    r, g, b = rgb_color
    assert 0 <= r <= 1.
    assert 0 <= g <= 1.
    assert 0 <= b <= 1.
    rh = int(r * 255.)
    gh = int(g * 255.)
    bh = int(b * 255.)
    return "0x%.02x%.02x%.02x" % (rh, gh, bh)


def export_edgedata_to_json(edge_hash, point_set):
    """ Export a set of points to a LineSegment buffergeometry

    Parameters
    ----------
    edge_hash
    point_set

    Returns
    -------
    str : a JSON string

    """
    # first build the array of point coordinates
    # edges are built as follows:
    # points_coordinates  =[P0x, P0y, P0z, P1x, P1y, P1z, P2x, P2y, etc.]
    points_coordinates = []
    for point in point_set:
        for coord in point:
            points_coordinates.append(coord)
    # then build the dictionary exported to json
    edges_data = {"metadata": {"version": 4.4,
                               "type": "BufferGeometry",
                               "generator": "pythonocc"},
                  "uuid": edge_hash,
                  "type": "BufferGeometry",
                  "data": {"attributes": {"position": {"itemSize": 3,
                                                       "type": "Float32Array",
                                                       "array": points_coordinates}
                                          }
                           }
                  }
    return json.dumps(edges_data)


# TODO: move control of color and appearance from HTML template to JSON
# TODO : separate the conversion from the IO
def _shape_to_json(shape,
                   filename,
                   export_edges=False,
                   color=(0.65, 0.65, 0.65),
                   specular_color=(1, 1, 1),
                   shininess=0.9,
                   transparency=0.,
                   line_color=(0, 0., 0.),
                   line_width=2.,
                   mesh_quality=1.):
    r"""Converts a shape to a JSON file representation

    Parameters
    ----------
    shape : OCC shape
    filename : str
        Full path to the file where the conversion is written
    export_edges : bool
    color
    specular_color
    shininess
    transparency
    line_color
    line_width
    mesh_quality

    Returns
    -------
    Nothing, it is a procedure, not a function

    """
    logger.info("Starting the conversion of a shape to JSON(%s)" % filename)
    _3js_shapes = {}
    _3js_edges = {}

    # if the shape is an edge or a wire, use the related functions
    if is_edge(shape):
        logger.debug("discretize an edge")
        pnts = discretize_edge(shape)
        edge_hash = "edg%s" % uuid.uuid4().hex
        str_to_write = export_edgedata_to_json(edge_hash, pnts)
        # edge_full_path = os.path.join(path, edge_hash + '.json')
        with open(filename, "w") as edge_file:
            edge_file.write(str_to_write)
        # store this edge hash
        _3js_edges[edge_hash] = [color, line_width]
        return True

    elif is_wire(shape):
        logger.debug("discretize a wire")
        pnts = discretize_wire(list(TopologyExplorer(shape).wires())[0])
        wire_hash = "wir%s" % uuid.uuid4().hex
        str_to_write = export_edgedata_to_json(wire_hash, pnts)
        # wire_full_path = os.path.join(path, wire_hash + '.json')
        with open(filename, "w") as wire_file:
            wire_file.write(str_to_write)
        # store this edge hash
        _3js_edges[wire_hash] = [color, line_width]
        return True

    shape_uuid = uuid.uuid4().hex
    shape_hash = "shp%s" % shape_uuid
    # tesselate
    tess = Tesselator(shape)
    tess.Compute(compute_edges=export_edges,
                 mesh_quality=mesh_quality,
                 uv_coords=False,
                 parallel=True)

    # export to 3JS
    # shape_full_path = os.path.join(path, shape_hash + '.json')
    # add this shape to the shape dict, sotres everything related to it
    _3js_shapes[shape_hash] = [export_edges,
                               color,
                               specular_color,
                               shininess,
                               transparency,
                               line_color,
                               line_width]
    # generate the mesh
    # tess.ExportShapeToThreejs(shape_hash, shape_full_path)
    # and also to JSON
    with open(filename, 'w') as json_file:
        json_file.write(tess.ExportShapeToThreejsJSONString(shape_uuid))

    # draw edges if necessary
    # if export_edges:
    #     # export each edge to a single json
    #     # get number of edges
    #     nbr_edges = tess.ObjGetEdgeCount()
    #     for i_edge in range(nbr_edges):
    #         # after that, the file can be appended
    #         str_to_write = ''
    #         edge_point_set = []
    #         nbr_vertices = tess.ObjEdgeGetVertexCount(i_edge)
    #         for i_vert in range(nbr_vertices):
    #             edge_point_set.append(tess.GetEdgeVertex(i_edge, i_vert))
    #         # write to file
    #         edge_hash = "edg%s" % uuid.uuid4().hex
    #         str_to_write += export_edgedata_to_json(edge_hash, edge_point_set)
    #         # create the file
    #         edge_full_path = os.path.join(path, edge_hash + '.json')
    #         with open(edge_full_path, "w") as edge_file:
    #             edge_file.write(str_to_write)
    #         # store this edge hash, with black color
    #         _3js_edges[hash] = [(0, 0, 0), line_width]
    logger.info("End of the conversion of a shape to JSON(%s)" % filename)


# def _convert_shape_stl(shape, filename):
#     r"""Write a shape to the converted file
#
#     NOT USED ANYMORE - DEPRECATED
#
#     Parameters
#     ----------
#     shape : OCC Shape
#         The input shape
#     filename : str
#         Path to the destination file
#
#     """
#     e = StlExporter(filename=filename, ascii_mode=False)
#     e.set_shape(shape)
#     e.write_file()
