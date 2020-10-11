# coding: utf-8

r"""Example of a car model"""

from car_assemblies import make_chassis_assembly
from osvcad.view import view_assembly, view_assembly_graph

assembly = make_chassis_assembly()
for k, v in assembly.anchors.items():
    print("%s : %s" % (k, v))

if __name__ == "__main__":
    view_assembly(assembly)
    view_assembly_graph(assembly)
