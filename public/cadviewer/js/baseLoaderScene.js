/**
 * Simple base class, which setups a simple scene which is used to 
 * demonstrate the different loaders. This create a scene, three
 * lights, and slowly rotates the model, around the z-axis
 */
function BaseLoaderScene(providedCamera, shouldAddLights, shouldRotate, updateMesh) {

  self = this;

  // setup some default elements
  this.scene = new THREE.Scene();
  this.stats = initStats();
  this.clock = new THREE.Clock();
  this.camera = providedCamera;
  this.withLights = (shouldAddLights !== undefined) ? shouldAddLights : true;
  this.shouldRotate = (shouldRotate !== undefined) ? shouldRotate : true;
  this.updateMesh = updateMesh

  // initialize basic renderer
  this.renderer = initRenderer({
    antialias: true
  });

  this.trackballControls = initTrackballControls(this.camera, this.renderer);

  /**
   * Start the render loop of the provided object
   * 
   * @param {Three.Object3D} mesh render this mesh or object
   * @param {*} camera render using the provided camera settings
   */
  this.render = function (mesh, camera) {
    self.scene.add(mesh);
    self.camera = camera;
    self.mesh = mesh;
    self._render();
  }

  /**
   * Interal function, called continously to render the scene
   */
  this._render = function () {
    self.stats.update();
    requestAnimationFrame(self._render);
    self.trackballControls.update(self.clock.getDelta());

    if (updateMesh) this.updateMesh(self.mesh)

    if (shouldRotate) {
      self.mesh.rotation.z += 0.01
    }

    self.renderer.render(self.scene, self.camera);
  }

  /**
   * Internal function, which adds a number of lights to the scene.
   */
  this._addLights = function () {
    var keyLight = new THREE.SpotLight(0xffffff);
    keyLight.position.set(00, 80, 80);
    keyLight.intensity = 2;
    keyLight.lookAt(new THREE.Vector3(0, 15, 0));
    keyLight.castShadow = true;
    keyLight.shadow.mapSize.height = 4096;
    keyLight.shadow.mapSize.width = 4096;
    this.scene.add(keyLight);

    var backlight1 = new THREE.SpotLight(0xaaaaaa);
    backlight1.position.set(150, 40, -20);
    backlight1.intensity = 0.5;
    backlight1.lookAt(new THREE.Vector3(0, 15, 0));
    this.scene.add(backlight1);

    var backlight2 = new THREE.SpotLight(0xaaaaaa);
    backlight2.position.set(-150, 40, -20);
    backlight2.intensity = 0.5;
    backlight2.lookAt(new THREE.Vector3(0, 15, 0));
    this.scene.add(backlight2);
  }

  // add the lights
  if (this.withLights) this._addLights();

}
