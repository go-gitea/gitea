// This is fugly as hell. I simply cut, pasted, and wrapped it with 
// a simple interface. Sorry! -wwwtyro
// https://github.com/wwwtyro/perlin.js

function Perlin(seed) {

// Alea random number generator.
//----------------------------------------------------------------------------//

    // From http://baagoe.com/en/RandomMusings/javascript/
    function Alea() {
        return (function(args) {
            // Johannes BaagÃ¸e <baagoe@baagoe.com>, 2010
            var s0 = 0;
            var s1 = 0;
            var s2 = 0;
            var c = 1;

            if (args.length == 0) {
                args = [+new Date];
            }
            var mash = Mash();
            s0 = mash(' ');
            s1 = mash(' ');
            s2 = mash(' ');

            for (var i = 0; i < args.length; i++) {
                s0 -= mash(args[i]);
                if (s0 < 0) {
                    s0 += 1;
                }
                s1 -= mash(args[i]);
                if (s1 < 0) {
                    s1 += 1;
                }
                s2 -= mash(args[i]);
                if (s2 < 0) {
                    s2 += 1;
                }
            }
            mash = null;

            var random = function() {
                var t = 2091639 * s0 + c * 2.3283064365386963e-10; // 2^-32
                s0 = s1;
                s1 = s2;
                return s2 = t - (c = t | 0);
            };
            random.uint32 = function() {
                return random() * 0x100000000; // 2^32
            };
            random.fract53 = function() {
                return random() +
                    (random() * 0x200000 | 0) * 1.1102230246251565e-16; // 2^-53
            };
            random.version = 'Alea 0.9';
            random.args = args;
            return random;

        } (Array.prototype.slice.call(arguments)));
    };

    // From http://baagoe.com/en/RandomMusings/javascript/
    // Johannes BaagÃ¸e <baagoe@baagoe.com>, 2010
    function Mash() {
        var n = 0xefc8249d;

        var mash = function(data) {
            data = data.toString();
            for (var i = 0; i < data.length; i++) {
                n += data.charCodeAt(i);
                var h = 0.02519603282416938 * n;
                n = h >>> 0;
                h -= n;
                h *= n;
                n = h >>> 0;
                h -= n;
                n += h * 0x100000000; // 2^32
            }
            return (n >>> 0) * 2.3283064365386963e-10; // 2^-32
        };

        mash.version = 'Mash 0.9';
        return mash;
    }

// Simplex perlin noise.
//----------------------------------------------------------------------------//

    // Ported from Stefan Gustavson's java implementation
    // http://staffwww.itn.liu.se/~stegu/simplexnoise/simplexnoise.pdf
    // Read Stefan's excellent paper for details on how this code works.
    //
    // Sean McCullough banksean@gmail.com

    /**
     * You can pass in a random number generator object if you like.
     * It is assumed to have a random() method.
     */
    var SimplexNoise = function(r) {
        if (r == undefined) r = Math;
        this.grad3 = [[1,1,0],[-1,1,0],[1,-1,0],[-1,-1,0],
            [1,0,1],[-1,0,1],[1,0,-1],[-1,0,-1],
            [0,1,1],[0,-1,1],[0,1,-1],[0,-1,-1]];
        this.p = [];
        for (var i=0; i<256; i++) {
            this.p[i] = Math.floor(r.random()*256);
        }
        // To remove the need for index wrapping, double the permutation table length
        this.perm = [];
        for(var i=0; i<512; i++) {
            this.perm[i]=this.p[i & 255];
        }

        // A lookup table to traverse the simplex around a given point in 4D.
        // Details can be found where this table is used, in the 4D noise method.
        this.simplex = [
            [0,1,2,3],[0,1,3,2],[0,0,0,0],[0,2,3,1],[0,0,0,0],[0,0,0,0],[0,0,0,0],[1,2,3,0],
            [0,2,1,3],[0,0,0,0],[0,3,1,2],[0,3,2,1],[0,0,0,0],[0,0,0,0],[0,0,0,0],[1,3,2,0],
            [0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],
            [1,2,0,3],[0,0,0,0],[1,3,0,2],[0,0,0,0],[0,0,0,0],[0,0,0,0],[2,3,0,1],[2,3,1,0],
            [1,0,2,3],[1,0,3,2],[0,0,0,0],[0,0,0,0],[0,0,0,0],[2,0,3,1],[0,0,0,0],[2,1,3,0],
            [0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],[0,0,0,0],
            [2,0,1,3],[0,0,0,0],[0,0,0,0],[0,0,0,0],[3,0,1,2],[3,0,2,1],[0,0,0,0],[3,1,2,0],
            [2,1,0,3],[0,0,0,0],[0,0,0,0],[0,0,0,0],[3,1,0,2],[0,0,0,0],[3,2,0,1],[3,2,1,0]];
    };

    SimplexNoise.prototype.dot = function(g, x, y) {
        return g[0]*x + g[1]*y;
    };

    SimplexNoise.prototype.noise = function(xin, yin) {
        var n0, n1, n2; // Noise contributions from the three corners
        // Skew the input space to determine which simplex cell we're in
        var F2 = 0.5*(Math.sqrt(3.0)-1.0);
        var s = (xin+yin)*F2; // Hairy factor for 2D
        var i = Math.floor(xin+s);
        var j = Math.floor(yin+s);
        var G2 = (3.0-Math.sqrt(3.0))/6.0;
        var t = (i+j)*G2;
        var X0 = i-t; // Unskew the cell origin back to (x,y) space
        var Y0 = j-t;
        var x0 = xin-X0; // The x,y distances from the cell origin
        var y0 = yin-Y0;
        // For the 2D case, the simplex shape is an equilateral triangle.
        // Determine which simplex we are in.
        var i1, j1; // Offsets for second (middle) corner of simplex in (i,j) coords
        if(x0>y0) {i1=1; j1=0;} // lower triangle, XY order: (0,0)->(1,0)->(1,1)
        else {i1=0; j1=1;}      // upper triangle, YX order: (0,0)->(0,1)->(1,1)
        // A step of (1,0) in (i,j) means a step of (1-c,-c) in (x,y), and
        // a step of (0,1) in (i,j) means a step of (-c,1-c) in (x,y), where
        // c = (3-sqrt(3))/6
        var x1 = x0 - i1 + G2; // Offsets for middle corner in (x,y) unskewed coords
        var y1 = y0 - j1 + G2;
        var x2 = x0 - 1.0 + 2.0 * G2; // Offsets for last corner in (x,y) unskewed coords
        var y2 = y0 - 1.0 + 2.0 * G2;
        // Work out the hashed gradient indices of the three simplex corners
        var ii = i & 255;
        var jj = j & 255;
        var gi0 = this.perm[ii+this.perm[jj]] % 12;
        var gi1 = this.perm[ii+i1+this.perm[jj+j1]] % 12;
        var gi2 = this.perm[ii+1+this.perm[jj+1]] % 12;
        // Calculate the contribution from the three corners
        var t0 = 0.5 - x0*x0-y0*y0;
        if(t0<0) n0 = 0.0;
        else {
            t0 *= t0;
            n0 = t0 * t0 * this.dot(this.grad3[gi0], x0, y0);  // (x,y) of grad3 used for 2D gradient
        }
        var t1 = 0.5 - x1*x1-y1*y1;
        if(t1<0) n1 = 0.0;
        else {
            t1 *= t1;
            n1 = t1 * t1 * this.dot(this.grad3[gi1], x1, y1);
        }
        var t2 = 0.5 - x2*x2-y2*y2;
        if(t2<0) n2 = 0.0;
        else {
            t2 *= t2;
            n2 = t2 * t2 * this.dot(this.grad3[gi2], x2, y2);
        }
        // Add contributions from each corner to get the final noise value.
        // The result is scaled to return values in the interval [-1,1].
        return 70.0 * (n0 + n1 + n2);
    };

    // 3D simplex noise 
    SimplexNoise.prototype.noise3d = function(xin, yin, zin) {
        var n0, n1, n2, n3; // Noise contributions from the four corners
        // Skew the input space to determine which simplex cell we're in
        var F3 = 1.0/3.0;
        var s = (xin+yin+zin)*F3; // Very nice and simple skew factor for 3D
        var i = Math.floor(xin+s);
        var j = Math.floor(yin+s);
        var k = Math.floor(zin+s);
        var G3 = 1.0/6.0; // Very nice and simple unskew factor, too
        var t = (i+j+k)*G3;
        var X0 = i-t; // Unskew the cell origin back to (x,y,z) space
        var Y0 = j-t;
        var Z0 = k-t;
        var x0 = xin-X0; // The x,y,z distances from the cell origin
        var y0 = yin-Y0;
        var z0 = zin-Z0;
        // For the 3D case, the simplex shape is a slightly irregular tetrahedron.
        // Determine which simplex we are in.
        var i1, j1, k1; // Offsets for second corner of simplex in (i,j,k) coords
        var i2, j2, k2; // Offsets for third corner of simplex in (i,j,k) coords
        if(x0>=y0) {
            if(y0>=z0)
            { i1=1; j1=0; k1=0; i2=1; j2=1; k2=0; } // X Y Z order
            else if(x0>=z0) { i1=1; j1=0; k1=0; i2=1; j2=0; k2=1; } // X Z Y order
            else { i1=0; j1=0; k1=1; i2=1; j2=0; k2=1; } // Z X Y order
        }
        else { // x0<y0
            if(y0<z0) { i1=0; j1=0; k1=1; i2=0; j2=1; k2=1; } // Z Y X order
            else if(x0<z0) { i1=0; j1=1; k1=0; i2=0; j2=1; k2=1; } // Y Z X order
            else { i1=0; j1=1; k1=0; i2=1; j2=1; k2=0; } // Y X Z order
        }
        // A step of (1,0,0) in (i,j,k) means a step of (1-c,-c,-c) in (x,y,z),
        // a step of (0,1,0) in (i,j,k) means a step of (-c,1-c,-c) in (x,y,z), and
        // a step of (0,0,1) in (i,j,k) means a step of (-c,-c,1-c) in (x,y,z), where
        // c = 1/6.
        var x1 = x0 - i1 + G3; // Offsets for second corner in (x,y,z) coords
        var y1 = y0 - j1 + G3;
        var z1 = z0 - k1 + G3;
        var x2 = x0 - i2 + 2.0*G3; // Offsets for third corner in (x,y,z) coords
        var y2 = y0 - j2 + 2.0*G3;
        var z2 = z0 - k2 + 2.0*G3;
        var x3 = x0 - 1.0 + 3.0*G3; // Offsets for last corner in (x,y,z) coords
        var y3 = y0 - 1.0 + 3.0*G3;
        var z3 = z0 - 1.0 + 3.0*G3;
        // Work out the hashed gradient indices of the four simplex corners
        var ii = i & 255;
        var jj = j & 255;
        var kk = k & 255;
        var gi0 = this.perm[ii+this.perm[jj+this.perm[kk]]] % 12;
        var gi1 = this.perm[ii+i1+this.perm[jj+j1+this.perm[kk+k1]]] % 12;
        var gi2 = this.perm[ii+i2+this.perm[jj+j2+this.perm[kk+k2]]] % 12;
        var gi3 = this.perm[ii+1+this.perm[jj+1+this.perm[kk+1]]] % 12;
        // Calculate the contribution from the four corners
        var t0 = 0.6 - x0*x0 - y0*y0 - z0*z0;
        if(t0<0) n0 = 0.0;
        else {
            t0 *= t0;
            n0 = t0 * t0 * this.dot(this.grad3[gi0], x0, y0, z0);
        }
        var t1 = 0.6 - x1*x1 - y1*y1 - z1*z1;
        if(t1<0) n1 = 0.0;
        else {
            t1 *= t1;
            n1 = t1 * t1 * this.dot(this.grad3[gi1], x1, y1, z1);
        }
        var t2 = 0.6 - x2*x2 - y2*y2 - z2*z2;
        if(t2<0) n2 = 0.0;
        else {
            t2 *= t2;
            n2 = t2 * t2 * this.dot(this.grad3[gi2], x2, y2, z2);
        }
        var t3 = 0.6 - x3*x3 - y3*y3 - z3*z3;
        if(t3<0) n3 = 0.0;
        else {
            t3 *= t3;
            n3 = t3 * t3 * this.dot(this.grad3[gi3], x3, y3, z3);
        }
        // Add contributions from each corner to get the final noise value.
        // The result is scaled to stay just inside [-1,1]
        return 32.0*(n0 + n1 + n2 + n3);
    };

// Classic Perlin noise, 3D version 
//----------------------------------------------------------------------------//

    var ClassicalNoise = function(r) { // Classic Perlin noise in 3D, for comparison 
        if (r == undefined) r = Math;
        this.grad3 = [[1,1,0],[-1,1,0],[1,-1,0],[-1,-1,0],
            [1,0,1],[-1,0,1],[1,0,-1],[-1,0,-1],
            [0,1,1],[0,-1,1],[0,1,-1],[0,-1,-1]];
        this.p = [];
        for (var i=0; i<256; i++) {
            this.p[i] = Math.floor(r.random()*256);
        }
        // To remove the need for index wrapping, double the permutation table length
        this.perm = [];
        for(var i=0; i<512; i++) {
            this.perm[i]=this.p[i & 255];
        }
    };

    ClassicalNoise.prototype.dot = function(g, x, y, z) {
        return g[0]*x + g[1]*y + g[2]*z;
    };

    ClassicalNoise.prototype.mix = function(a, b, t) {
        return (1.0-t)*a + t*b;
    };

    ClassicalNoise.prototype.fade = function(t) {
        return t*t*t*(t*(t*6.0-15.0)+10.0);
    };

    ClassicalNoise.prototype.noise = function(x, y, z) {
        // Find unit grid cell containing point
        var X = Math.floor(x);
        var Y = Math.floor(y);
        var Z = Math.floor(z);

        // Get relative xyz coordinates of point within that cell
        x = x - X;
        y = y - Y;
        z = z - Z;

        // Wrap the integer cells at 255 (smaller integer period can be introduced here)
        X = X & 255;
        Y = Y & 255;
        Z = Z & 255;

        // Calculate a set of eight hashed gradient indices
        var gi000 = this.perm[X+this.perm[Y+this.perm[Z]]] % 12;
        var gi001 = this.perm[X+this.perm[Y+this.perm[Z+1]]] % 12;
        var gi010 = this.perm[X+this.perm[Y+1+this.perm[Z]]] % 12;
        var gi011 = this.perm[X+this.perm[Y+1+this.perm[Z+1]]] % 12;
        var gi100 = this.perm[X+1+this.perm[Y+this.perm[Z]]] % 12;
        var gi101 = this.perm[X+1+this.perm[Y+this.perm[Z+1]]] % 12;
        var gi110 = this.perm[X+1+this.perm[Y+1+this.perm[Z]]] % 12;
        var gi111 = this.perm[X+1+this.perm[Y+1+this.perm[Z+1]]] % 12;

        // The gradients of each corner are now:
        // g000 = grad3[gi000];
        // g001 = grad3[gi001];
        // g010 = grad3[gi010];
        // g011 = grad3[gi011];
        // g100 = grad3[gi100];
        // g101 = grad3[gi101];
        // g110 = grad3[gi110];
        // g111 = grad3[gi111];
        // Calculate noise contributions from each of the eight corners
        var n000= this.dot(this.grad3[gi000], x, y, z);
        var n100= this.dot(this.grad3[gi100], x-1, y, z);
        var n010= this.dot(this.grad3[gi010], x, y-1, z);
        var n110= this.dot(this.grad3[gi110], x-1, y-1, z);
        var n001= this.dot(this.grad3[gi001], x, y, z-1);
        var n101= this.dot(this.grad3[gi101], x-1, y, z-1);
        var n011= this.dot(this.grad3[gi011], x, y-1, z-1);
        var n111= this.dot(this.grad3[gi111], x-1, y-1, z-1);
        // Compute the fade curve value for each of x, y, z
        var u = this.fade(x);
        var v = this.fade(y);
        var w = this.fade(z);
        // Interpolate along x the contributions from each of the corners
        var nx00 = this.mix(n000, n100, u);
        var nx01 = this.mix(n001, n101, u);
        var nx10 = this.mix(n010, n110, u);
        var nx11 = this.mix(n011, n111, u);
        // Interpolate the four results along y
        var nxy0 = this.mix(nx00, nx10, v);
        var nxy1 = this.mix(nx01, nx11, v);
        // Interpolate the two last results along z
        var nxyz = this.mix(nxy0, nxy1, w);

        return nxyz;
    };


//----------------------------------------------------------------------------//


    var rand = {};
    rand.random = new Alea(seed);
    var noise = new ClassicalNoise(rand);

    this.noise = function (x, y, z) {
        return 0.5 * noise.noise(x, y, z) + 0.5;
    }

}