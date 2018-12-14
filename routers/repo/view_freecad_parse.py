#!/usr/bin/env python
# coding: utf-8

import xml.etree.ElementTree


def xml_root(xml_filepath):
    r"""Get the XML root element of an XML file

    Parameters
    ----------
    xml_filepath : str
        Full path to the XML file

    Returns
    -------
    XML root element

    Raises
    ------
    FileNotFoundError if xml_filepath points to a file that does not exists

    """
    return xml.etree.ElementTree.parse(xml_filepath).getroot()


def list_objects(doc_root, container="Objects"):
    r"""

    Parameters
    ----------
    doc_root : xml element
        Root element of a Document.xml file
    container : str
        Name of the XML container for the objects

    Returns
    -------
    List of XML element corresponding to the Objects

    """
    if container not in ["Objects", "ObjectData"]:
        raise ValueError("Unknown container")
    objects = []
    for objects_entry in doc_root.findall(container):
        for object_entry in objects_entry.findall('Object'):
            objects.append(object_entry)
    return objects


def name_file(doc_root):
    r"""

    Parameters
    ----------
    doc_root : xml element
        Root element of the Document.xml file

    Returns
    -------
    list of tuples
        List of (name, file) tuples from the Document.xml file

    """
    name_file_tuples = []
    lo = list_objects(doc_root, container="Objects")
    lod = list_objects(doc_root, container="ObjectData")
    for lo_ in lo:
        name = lo_.attrib['name']
        object_data_entry = filter(lambda e: e.attrib['name'] == name, lod)
        for de in object_data_entry:
            for properties in de.findall("Properties"):
                for prop in filter(lambda e: e.attrib['name'] == "Shape",
                                   properties.findall("Property")):
                    for part in prop.findall("Part"):
                        name_file_tuples.append((name, part.attrib['file']))
    return name_file_tuples


def name_visibility(guidoc_root, name):
    r"""Using GuiDocument.xml, determine the visibility of an object

    Parameters
    ----------
    guidoc_root : xml element
        XML root element of the GuiDocument.xml file
    name : str
        Object name

    Returns
    -------
    Bool
        True if visible, False otherwise

    """
    for vpd in guidoc_root.findall("ViewProviderData"):
        for vp in filter(lambda x: x.attrib['name'] == name,
                         vpd.findall("ViewProvider")):
            for ps in vp.findall("Properties"):
                for p in filter(lambda x: x.attrib['name'] == "Visibility",
                                ps.findall("Property")):
                    for b in p.findall("Bool"):
                        if b.attrib["value"] == "true":
                            return True
                        else:
                            return False


def name_file_visibility(name_file_tuples, guidoc_root):
    r"""Build a list of name, file, visibility tuples

    Parameters
    ----------
    name_file_tuples : list of tuples
        List of (name, file) tuples
    guidoc_root : xml element
        XML root element of the GuiDocument.xml file

    Returns
    -------
    list of tuples
        List of (name, file, visibility) tuples

    """
    name_file_visibility_tuples = []
    for name, file in name_file_tuples:
        visibility = name_visibility(guidoc_root, name)
        name_file_visibility_tuples.append((name, file, visibility))
    return name_file_visibility_tuples
