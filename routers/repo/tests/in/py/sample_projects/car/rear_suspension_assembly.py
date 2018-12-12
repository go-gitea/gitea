# coding: utf-8

r"""Tabby rear suspension assembly"""

from car_assemblies import make_rear_suspension_assembly
from osvcad.view import view_assembly, view_assembly_graph

assembly = make_rear_suspension_assembly()

if __name__ == "__main__":
    view_assembly(assembly)
    view_assembly_graph(assembly)
