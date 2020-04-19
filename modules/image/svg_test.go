// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package image

import (
	"bytes"
	"io"
	"testing"

	minify "github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/svg"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSVG(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "ariaData",
			input: `<svg version="1" id="cat" viewBox="0 0 720 800" aria-labelledby="catTitle catDesc" role="img">
	<title id="catTitle" arial-dontallow="nope">Pixels, My Super-friendly Cat</title>
	<desc id="catDesc">An illustrated gray cat with bright green blinking eyes.</desc>
	<path id="tail" data-name="tail" class="cls-1" d="M545.9,695.9c8,28.2,23.2,42.3,27.2,46.9,21.4,24.1,41.5,40.2,81.1,42.9s65.4-14.2,60.8-26.8-23.1-9.1-51.3-8.3c-35.2.9-66.6-31.3-74.8-63.9s-7.9-63.8-36.8-85.5c-44.1-33-135.6-7.1-159.8-3.4s-48.4,52.5-9.6,45.1,91.4-23.1,123.2-12.7C537.8,640.4,537.9,667.7,545.9,695.9Z" transform="translate(-9.7 -9.3)"/>
	<g id="body">
		<path id="bg" class="cls-2" d="M447.9,502.1c2.1,151.7-108.3,167-216.5,167S9.7,663.8,9.7,510.9,85,242.9,231.3,241,445.8,350.4,447.9,502.1h0Z" transform="translate(-9.7 -9.3)"/>
		<g id="leftleg" datas-dontallow="nope">
			<path id="leg" class="cls-1" d="M195.6,671.5c-34.2-7.7-40.6-95.6-53.3-191-12-90-90.1-177.2-55.1-177.2s145.7,12,151.4,87.7S261.5,686.5,195.6,671.5Z" transform="translate(-9.7 -9.3)"/>
			<path id="foot" class="cls-3" d="M172.2,688.1c31.6,2.1,56.6-8.7,59.8-32.4s-22.1-49.5-27.3-24.3c25-16.4-39.1-29.4-27.6-3.9,14-24.9-49.6-19.2-31.9-.1-6.5-27.2-35.6,8.2-30.1,29.3C121.5,681.8,140.5,686,172.2,688.1Z" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="rightleg">
			<path id="leg-2" data-name="leg" class="cls-1" d="M260.4,670.4c42.4-9.2,48.7-87.7,53.9-185.2,5.1-96,98.2-176.1,63.1-176.1s-164,15.7-164,111.8C213.4,420.9,199.1,683.7,260.4,670.4Z" transform="translate(-9.7 -9.3)"/>
			<path id="foot-2" data-name="foot" class="cls-3" d="M279.4,689.8c-31.7,2-56.6-9-59.6-32.6s22.3-49.4,27.4-24.1c-24.9-16.5,39.2-29.2,27.6-3.8-13.9-25,49.7-18.9,31.9,0,6.6-27.1,35.6,8.4,30,29.4-6.7,25-25.7,29.1-57.3,31.1h0Z" transform="translate(-9.7 -9.3)"/>
		</g>
		<path id="tuft" aria-haspopup="false" class="cls-3" d="M80,331.2c3.5,9.5,1.2,28.9,4.3,32.7s31.5-30,43-20.6c10.7,8.7,1.7,55.9,12.9,64.5,10.1,7.7,32.1-50.6,52.5-38.7,24.9,14.6,34.1,49.9,49,49.9,18.3,0,7.5-49.5,24.1-53.3s46.1,52.6,60.2,45.6c4.8-2.4,3-50.4,12-57.6,8.7-6.9,30.5,22.4,33.5,18.9,3.7-4.1.1-23.1,8.6-36.1,3.4-5.2,18.9-2.6,28.8-.4a3.46,3.46,0,0,0,3.7-5.2c-19.6-30.8-100-147.4-184.2-147.4-93.3,0-150.9,86.8-178.1,141.6a3.43,3.43,0,0,0,3.6,4.9C63,328.4,78.4,326.6,80,331.2Z" transform="translate(-9.7 -9.3)"/>
	</g>
	<g id="head">
		<path id="collar" class="cls-4" d="M367,231.1c5.7,36.1-4.7,71-97.8,85.6s-184-18.5-189.7-54.5,16.7-17.3,109.8-31.9,172-35.3,177.7.8" transform="translate(-9.7 -9.3)"/>
		<g id="bg-2" data-name="bg">
			<path class="cls-1" d="M362.5,229.5C339.7,279,273.1,299.4,225,300c-60.6.7-134.7-29.5-153.5-86.4C45.6,135.4,132.2,32.6,225,35.8c96.1,3.4,171.7,119.4,137.5,193.7" transform="translate(-9.7 -9.3)"/>
			<path class="cls-5" d="M362.5,229.5C339.7,279,273.1,299.4,225,300c-60.6.7-134.7-29.5-153.5-86.4C45.6,135.4,132.2,32.6,225,35.8,321.1,39.2,396.7,155.2,362.5,229.5Z" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="leftear" aria-label="Left Ear">
			<path id="outer" class="cls-1" d="M92.7,117c-2.6,4.7-14.7-16.1-16.5-45-3.3-27.7,3.7-63.4,5.4-62C80.7,8,117,10,143,20c27.5,8.9,44.7,25.7,39.5,27.1-30,23.4-59.9,46.6-89.8,69.9" transform="translate(-9.7 -9.3)"/>
			<path id="inner" class="cls-6" d="M105.8,106.9C103.9,110.3,95.3,95.5,94,75c-2.3-19.6,2.6-44.9,3.8-44-0.6-1.4,25.1,0,43.6,7.1,19.5,6.3,31.7,18.2,28,19.2q-31.8,24.9-63.6,49.6" transform="translate(-9.7 -9.3)"/>
		</g>
		<path id="mask" class="cls-2" d="M338.4,142.5c-2.2,3.3,19.4,19.6,17.2,23.2s-24.3-7.8-25.8-5.2c-1.9,3.3,33.4,24.1,31,29.2-2.3,4.9-34-14.4-84.3-18.1a141.76,141.76,0,0,1-16.4-2.1,91.21,91.21,0,0,1-13.7-3.9c-19.8-6.9-27.7-10.6-32.7-12-19.3-5.7-26.8,11.3-68.1,22.4-18.8,5-37.9,9.7-54.4,0-2.1-1.3-13.6-8.3-16.7-21.1-0.9-3.6-2.8-15.2,10.5-34C146.3,34.3,216.5,34,217.3,34a131.52,131.52,0,0,1,58.4,14.3c-7.6,4.9-11.2,9.5-9,10.1,21.5,16.5,43.1,33,64.6,49.5,0.9,1.7,3.6-1.3,6.3-7.3,19.3,30.5,22.1,41.5,18.9,44.3-3.8,3.6-16.4-4.8-18.1-2.4" transform="translate(-9.7 -9.3)"/>
		<g id="rightear">
			<path id="outer-2" data-name="outer" class="cls-2" d="M344.9,119.9c2.6,4.7,14.7-16.1,16.5-45,3.3-27.7-3.7-63.4-5.4-62,0.9-2-35.4,0-61.4,10-27.5,8.9-44.7,25.7-39.5,27.1q44.85,35,89.8,69.9" transform="translate(-9.7 -9.3)"/>
			<path id="inner-2" data-name="inner" class="cls-6" d="M343.5,76.2a77.83,77.83,0,0,1-5.6,24.6c-15.1-20.3-36-39.8-61-52.4a82,82,0,0,1,19.2-9.1c18.5-7.1,44.2-8.5,43.6-7.1,1.2-.9,6.1,24.4,3.8,44" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="nose">
			<path class="cls-7" d="M205.1,201.8l-10.6-18.3a9,9,0,0,1,7.7-13.4h21.2a8.9,8.9,0,0,1,7.7,13.4l-10.6,18.3a8.91,8.91,0,0,1-15.4,0" transform="translate(-9.7 -9.3)"/>
			<path class="cls-6" d="M194.2,175.1a9,9,0,0,0,.3,8.4l10.6,18.3a8.92,8.92,0,0,0,15.5,0l8.7-15c-5.8-6.2-19.3-10.1-35.1-11.7" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="mouth">
			<path class="cls-8" d="M166.7,260.4c-24.4,0-44.1-25-44.1-55.9m88.2,0c0,30.9-19.7,55.9-44.1,55.9m89.9,0c24.4,0,44.1-25,44.1-55.9m-88.2,0c0,30.9,19.7,55.9,44.1,55.9" transform="translate(-9.7 -9.3)"/>
			<path class="cls-9" d="M300.7,204.5a65.16,65.16,0,0,1-8,32" transform="translate(-9.7 -9.3)"/>
		</g>
		<path id="wiskers" class="cls-10" d="M188.7,198.4c0-12.9-72.7-23.3-162.6-23.3m162.6,36.2c0-7.1-65.8-12.9-147.1-12.9m196,1.3c1.4-12.8,74.8-15.6,164.1-6.2m-165.4,19c0.7-7.1,66.8-5.9,147.6,2.6" transform="translate(-9.7 -9.3)"/>
		<g id="lefteye" class="eye">
			<path id="iris" class="cls-4" d="M188.6,141.5s-18.3,12.3-35.8,7.9-30-15.2-27.7-24c1.5-6,9.6-9.6,20.2-9.8a59.5,59.5,0,0,1,15.7,1.9,35.75,35.75,0,0,1,12.5,6.2,60,60,0,0,1,15.1,17.8" transform="translate(-9.7 -9.3)"/>
			<path class="cls-11" d="M125.1,123.6c1.5-6,9.6-9.6,20.1-9.8a59.5,59.5,0,0,1,15.7,1.9,35.75,35.75,0,0,1,12.5,6.2,59.47,59.47,0,0,1,15.2,17.8" transform="translate(-9.7 -9.3)"/>
			<path id="pupil" class="cls-12" d="M172.9,124.3c-2.3,9.2-10.7,15-18.7,13s-12.5-11.1-10.2-20.4a22.39,22.39,0,0,1,1.1-3.1,59.5,59.5,0,0,1,15.7,1.9,35.75,35.75,0,0,1,12.5,6.2,8.6,8.6,0,0,1-.4,2.4" transform="translate(-9.7 -9.3)"/>
			<path id="eyelash" class="cls-13" d="M124.9,121.5c-7.6,2.6-17.1-4.7-21.1-16.3m33.6,9.5c-7.5,2.9-17.3-4-21.7-15.5m36.7,14.6c-8.1-.1-14.5-10.2-14.3-22.6" transform="translate(-9.7 -9.3)"/>
			<path id="reflection" class="cls-14" d="M156.8,122c0,3.6-2.6,6.4-5.8,6.4s-5.8-2.9-5.8-6.4,2.6-6.4,5.8-6.4,5.8,2.9,5.8,6.4" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="righteye" class="eye">
			<path id="iris-2" data-name="iris" class="cls-4" d="M241.4,143.6s18.5,11.9,36,7.1,29.6-15.8,27.2-24.6c-1.7-6-9.8-9.4-20.3-9.4a59.21,59.21,0,0,0-15.6,2.2,37.44,37.44,0,0,0-12.4,6.4,60.14,60.14,0,0,0-14.9,18.3" transform="translate(-9.7 -9.3)"/>
			<path id="lid" class="cls-11" d="M304.5,124.4c-1.7-6-9.8-9.4-20.3-9.4a59.21,59.21,0,0,0-15.6,2.2,37.44,37.44,0,0,0-12.4,6.4,61.21,61.21,0,0,0-14.9,18.1" transform="translate(-9.7 -9.3)"/>
			<path id="pupil-2" data-name="pupil" class="cls-12" d="M256.7,126.1c2.5,9.2,11,14.8,18.9,12.6s12.3-11.4,9.8-20.6a16.59,16.59,0,0,0-1.2-3.1,59.21,59.21,0,0,0-15.6,2.2,37.44,37.44,0,0,0-12.4,6.4,9.23,9.23,0,0,0,.5,2.5" transform="translate(-9.7 -9.3)"/>
			<path id="eyelash-2" data-name="eyelash" class="cls-13" d="M302.9,122.3c7.7,2.5,17-5,20.8-16.8M292,115.7c7.6,2.8,17.2-4.4,21.4-16M277,115.1c8.1-.3,14.3-10.5,13.9-22.8" transform="translate(-9.7 -9.3)"/>
			<path id="reflection-2" data-name="reflection" class="cls-14" d="M271.1,127.1c0,3.6-2.6,6.5-5.8,6.5s-5.8-2.9-5.8-6.5,2.6-6.4,5.8-6.4,5.8,2.9,5.8,6.4" transform="translate(-9.7 -9.3)"/>
		</g>
	</g>
</svg>`,
			/* Adjustement from https://github.com/darylldoyle/svg-sanitizer base
			   Diff:
			   --- Expected
			   +++ Actual
			   @@ -1,2 +1,2 @@
			   -<svg id="cat" viewBox="0 0 720 800" aria-labelledby="catTitle catDesc" role="img" version="1">
			   +<svg version="1" id="cat" viewbox="0 0 720 800" aria-labelledby="catTitle catDesc" role="img">
			*/
			want: `<svg version="1" id="cat" viewbox="0 0 720 800" aria-labelledby="catTitle catDesc" role="img">
	<title id="catTitle">Pixels, My Super-friendly Cat</title>
	<desc id="catDesc">An illustrated gray cat with bright green blinking eyes.</desc>
	<path id="tail" data-name="tail" class="cls-1" d="M545.9,695.9c8,28.2,23.2,42.3,27.2,46.9,21.4,24.1,41.5,40.2,81.1,42.9s65.4-14.2,60.8-26.8-23.1-9.1-51.3-8.3c-35.2.9-66.6-31.3-74.8-63.9s-7.9-63.8-36.8-85.5c-44.1-33-135.6-7.1-159.8-3.4s-48.4,52.5-9.6,45.1,91.4-23.1,123.2-12.7C537.8,640.4,537.9,667.7,545.9,695.9Z" transform="translate(-9.7 -9.3)"/>
	<g id="body">
		<path id="bg" class="cls-2" d="M447.9,502.1c2.1,151.7-108.3,167-216.5,167S9.7,663.8,9.7,510.9,85,242.9,231.3,241,445.8,350.4,447.9,502.1h0Z" transform="translate(-9.7 -9.3)"/>
		<g id="leftleg">
			<path id="leg" class="cls-1" d="M195.6,671.5c-34.2-7.7-40.6-95.6-53.3-191-12-90-90.1-177.2-55.1-177.2s145.7,12,151.4,87.7S261.5,686.5,195.6,671.5Z" transform="translate(-9.7 -9.3)"/>
			<path id="foot" class="cls-3" d="M172.2,688.1c31.6,2.1,56.6-8.7,59.8-32.4s-22.1-49.5-27.3-24.3c25-16.4-39.1-29.4-27.6-3.9,14-24.9-49.6-19.2-31.9-.1-6.5-27.2-35.6,8.2-30.1,29.3C121.5,681.8,140.5,686,172.2,688.1Z" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="rightleg">
			<path id="leg-2" data-name="leg" class="cls-1" d="M260.4,670.4c42.4-9.2,48.7-87.7,53.9-185.2,5.1-96,98.2-176.1,63.1-176.1s-164,15.7-164,111.8C213.4,420.9,199.1,683.7,260.4,670.4Z" transform="translate(-9.7 -9.3)"/>
			<path id="foot-2" data-name="foot" class="cls-3" d="M279.4,689.8c-31.7,2-56.6-9-59.6-32.6s22.3-49.4,27.4-24.1c-24.9-16.5,39.2-29.2,27.6-3.8-13.9-25,49.7-18.9,31.9,0,6.6-27.1,35.6,8.4,30,29.4-6.7,25-25.7,29.1-57.3,31.1h0Z" transform="translate(-9.7 -9.3)"/>
		</g>
		<path id="tuft" aria-haspopup="false" class="cls-3" d="M80,331.2c3.5,9.5,1.2,28.9,4.3,32.7s31.5-30,43-20.6c10.7,8.7,1.7,55.9,12.9,64.5,10.1,7.7,32.1-50.6,52.5-38.7,24.9,14.6,34.1,49.9,49,49.9,18.3,0,7.5-49.5,24.1-53.3s46.1,52.6,60.2,45.6c4.8-2.4,3-50.4,12-57.6,8.7-6.9,30.5,22.4,33.5,18.9,3.7-4.1.1-23.1,8.6-36.1,3.4-5.2,18.9-2.6,28.8-.4a3.46,3.46,0,0,0,3.7-5.2c-19.6-30.8-100-147.4-184.2-147.4-93.3,0-150.9,86.8-178.1,141.6a3.43,3.43,0,0,0,3.6,4.9C63,328.4,78.4,326.6,80,331.2Z" transform="translate(-9.7 -9.3)"/>
	</g>
	<g id="head">
		<path id="collar" class="cls-4" d="M367,231.1c5.7,36.1-4.7,71-97.8,85.6s-184-18.5-189.7-54.5,16.7-17.3,109.8-31.9,172-35.3,177.7.8" transform="translate(-9.7 -9.3)"/>
		<g id="bg-2" data-name="bg">
			<path class="cls-1" d="M362.5,229.5C339.7,279,273.1,299.4,225,300c-60.6.7-134.7-29.5-153.5-86.4C45.6,135.4,132.2,32.6,225,35.8c96.1,3.4,171.7,119.4,137.5,193.7" transform="translate(-9.7 -9.3)"/>
			<path class="cls-5" d="M362.5,229.5C339.7,279,273.1,299.4,225,300c-60.6.7-134.7-29.5-153.5-86.4C45.6,135.4,132.2,32.6,225,35.8,321.1,39.2,396.7,155.2,362.5,229.5Z" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="leftear" aria-label="Left Ear">
			<path id="outer" class="cls-1" d="M92.7,117c-2.6,4.7-14.7-16.1-16.5-45-3.3-27.7,3.7-63.4,5.4-62C80.7,8,117,10,143,20c27.5,8.9,44.7,25.7,39.5,27.1-30,23.4-59.9,46.6-89.8,69.9" transform="translate(-9.7 -9.3)"/>
			<path id="inner" class="cls-6" d="M105.8,106.9C103.9,110.3,95.3,95.5,94,75c-2.3-19.6,2.6-44.9,3.8-44-0.6-1.4,25.1,0,43.6,7.1,19.5,6.3,31.7,18.2,28,19.2q-31.8,24.9-63.6,49.6" transform="translate(-9.7 -9.3)"/>
		</g>
		<path id="mask" class="cls-2" d="M338.4,142.5c-2.2,3.3,19.4,19.6,17.2,23.2s-24.3-7.8-25.8-5.2c-1.9,3.3,33.4,24.1,31,29.2-2.3,4.9-34-14.4-84.3-18.1a141.76,141.76,0,0,1-16.4-2.1,91.21,91.21,0,0,1-13.7-3.9c-19.8-6.9-27.7-10.6-32.7-12-19.3-5.7-26.8,11.3-68.1,22.4-18.8,5-37.9,9.7-54.4,0-2.1-1.3-13.6-8.3-16.7-21.1-0.9-3.6-2.8-15.2,10.5-34C146.3,34.3,216.5,34,217.3,34a131.52,131.52,0,0,1,58.4,14.3c-7.6,4.9-11.2,9.5-9,10.1,21.5,16.5,43.1,33,64.6,49.5,0.9,1.7,3.6-1.3,6.3-7.3,19.3,30.5,22.1,41.5,18.9,44.3-3.8,3.6-16.4-4.8-18.1-2.4" transform="translate(-9.7 -9.3)"/>
		<g id="rightear">
			<path id="outer-2" data-name="outer" class="cls-2" d="M344.9,119.9c2.6,4.7,14.7-16.1,16.5-45,3.3-27.7-3.7-63.4-5.4-62,0.9-2-35.4,0-61.4,10-27.5,8.9-44.7,25.7-39.5,27.1q44.85,35,89.8,69.9" transform="translate(-9.7 -9.3)"/>
			<path id="inner-2" data-name="inner" class="cls-6" d="M343.5,76.2a77.83,77.83,0,0,1-5.6,24.6c-15.1-20.3-36-39.8-61-52.4a82,82,0,0,1,19.2-9.1c18.5-7.1,44.2-8.5,43.6-7.1,1.2-.9,6.1,24.4,3.8,44" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="nose">
			<path class="cls-7" d="M205.1,201.8l-10.6-18.3a9,9,0,0,1,7.7-13.4h21.2a8.9,8.9,0,0,1,7.7,13.4l-10.6,18.3a8.91,8.91,0,0,1-15.4,0" transform="translate(-9.7 -9.3)"/>
			<path class="cls-6" d="M194.2,175.1a9,9,0,0,0,.3,8.4l10.6,18.3a8.92,8.92,0,0,0,15.5,0l8.7-15c-5.8-6.2-19.3-10.1-35.1-11.7" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="mouth">
			<path class="cls-8" d="M166.7,260.4c-24.4,0-44.1-25-44.1-55.9m88.2,0c0,30.9-19.7,55.9-44.1,55.9m89.9,0c24.4,0,44.1-25,44.1-55.9m-88.2,0c0,30.9,19.7,55.9,44.1,55.9" transform="translate(-9.7 -9.3)"/>
			<path class="cls-9" d="M300.7,204.5a65.16,65.16,0,0,1-8,32" transform="translate(-9.7 -9.3)"/>
		</g>
		<path id="wiskers" class="cls-10" d="M188.7,198.4c0-12.9-72.7-23.3-162.6-23.3m162.6,36.2c0-7.1-65.8-12.9-147.1-12.9m196,1.3c1.4-12.8,74.8-15.6,164.1-6.2m-165.4,19c0.7-7.1,66.8-5.9,147.6,2.6" transform="translate(-9.7 -9.3)"/>
		<g id="lefteye" class="eye">
			<path id="iris" class="cls-4" d="M188.6,141.5s-18.3,12.3-35.8,7.9-30-15.2-27.7-24c1.5-6,9.6-9.6,20.2-9.8a59.5,59.5,0,0,1,15.7,1.9,35.75,35.75,0,0,1,12.5,6.2,60,60,0,0,1,15.1,17.8" transform="translate(-9.7 -9.3)"/>
			<path class="cls-11" d="M125.1,123.6c1.5-6,9.6-9.6,20.1-9.8a59.5,59.5,0,0,1,15.7,1.9,35.75,35.75,0,0,1,12.5,6.2,59.47,59.47,0,0,1,15.2,17.8" transform="translate(-9.7 -9.3)"/>
			<path id="pupil" class="cls-12" d="M172.9,124.3c-2.3,9.2-10.7,15-18.7,13s-12.5-11.1-10.2-20.4a22.39,22.39,0,0,1,1.1-3.1,59.5,59.5,0,0,1,15.7,1.9,35.75,35.75,0,0,1,12.5,6.2,8.6,8.6,0,0,1-.4,2.4" transform="translate(-9.7 -9.3)"/>
			<path id="eyelash" class="cls-13" d="M124.9,121.5c-7.6,2.6-17.1-4.7-21.1-16.3m33.6,9.5c-7.5,2.9-17.3-4-21.7-15.5m36.7,14.6c-8.1-.1-14.5-10.2-14.3-22.6" transform="translate(-9.7 -9.3)"/>
			<path id="reflection" class="cls-14" d="M156.8,122c0,3.6-2.6,6.4-5.8,6.4s-5.8-2.9-5.8-6.4,2.6-6.4,5.8-6.4,5.8,2.9,5.8,6.4" transform="translate(-9.7 -9.3)"/>
		</g>
		<g id="righteye" class="eye">
			<path id="iris-2" data-name="iris" class="cls-4" d="M241.4,143.6s18.5,11.9,36,7.1,29.6-15.8,27.2-24.6c-1.7-6-9.8-9.4-20.3-9.4a59.21,59.21,0,0,0-15.6,2.2,37.44,37.44,0,0,0-12.4,6.4,60.14,60.14,0,0,0-14.9,18.3" transform="translate(-9.7 -9.3)"/>
			<path id="lid" class="cls-11" d="M304.5,124.4c-1.7-6-9.8-9.4-20.3-9.4a59.21,59.21,0,0,0-15.6,2.2,37.44,37.44,0,0,0-12.4,6.4,61.21,61.21,0,0,0-14.9,18.1" transform="translate(-9.7 -9.3)"/>
			<path id="pupil-2" data-name="pupil" class="cls-12" d="M256.7,126.1c2.5,9.2,11,14.8,18.9,12.6s12.3-11.4,9.8-20.6a16.59,16.59,0,0,0-1.2-3.1,59.21,59.21,0,0,0-15.6,2.2,37.44,37.44,0,0,0-12.4,6.4,9.23,9.23,0,0,0,.5,2.5" transform="translate(-9.7 -9.3)"/>
			<path id="eyelash-2" data-name="eyelash" class="cls-13" d="M302.9,122.3c7.7,2.5,17-5,20.8-16.8M292,115.7c7.6,2.8,17.2-4.4,21.4-16M277,115.1c8.1-.3,14.3-10.5,13.9-22.8" transform="translate(-9.7 -9.3)"/>
			<path id="reflection-2" data-name="reflection" class="cls-14" d="M271.1,127.1c0,3.6-2.6,6.5-5.8,6.5s-5.8-2.9-5.8-6.5,2.6-6.4,5.8-6.4,5.8,2.9,5.8,6.4" transform="translate(-9.7 -9.3)"/>
		</g>
	</g>
</svg>`,
		},
		{
			name: "badXmlTestOne",
			input: `<?xml version="1.0" encoding="utf-8"?>
<!-- Generator: Adobe Illustrator 16.0.0, SVG Export Plug-In . SVG Version: 6.00 Build 0)  -->
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
	 width="600px" height="600px" viewBox="0 0 600 600" enable-background="new 0 0 600 600" xml:space="preserve">
<line onload="alert(2)" fill="none" stroke="#000000" stroke-miterlimit="10" x1="119" y1="84.5" x2="454" y2="84.5"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="111.212" y1="102.852" x2="112.032" y2="476.623"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="198.917" y1="510.229" x2="486.622" y2="501.213">
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="484.163" y1="442.196" x2="89.901" y2="60.229"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="101.376" y1="478.262" x2="443.18" y2="75.803"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="457.114" y1="126.623" x2="458.753" y2="363.508"/>
<this>shouldn't be here</this>
<script>alert(1);</script>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="541.54" y1="299.573" x2="543.179" y2="536.458"/>
</svg>`,
			/* Adjustement from https://github.com/darylldoyle/svg-sanitizer base
			--- Expected
			+++ Actual
			@@ -1,2 +1,9 @@
			-<svg id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewbox="0 0 600 600" xml:space="preserve">
			+<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" width="600px" height="600px" viewbox="0 0 600 600" xml:space="preserve">
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="119" y1="84.5" x2="454" y2="84.5"/>
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="111.212" y1="102.852" x2="112.032" y2="476.623"/>
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="198.917" y1="510.229" x2="486.622" y2="501.213">
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="484.163" y1="442.196" x2="89.901" y2="60.229"/>
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="101.376" y1="478.262" x2="443.18" y2="75.803"/>
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="457.114" y1="126.623" x2="458.753" y2="363.508"/>
			+<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="541.54" y1="299.573" x2="543.179" y2="536.458"/>
			*/
			want: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" width="600px" height="600px" viewbox="0 0 600 600" xml:space="preserve">
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="119" y1="84.5" x2="454" y2="84.5"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="111.212" y1="102.852" x2="112.032" y2="476.623"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="198.917" y1="510.229" x2="486.622" y2="501.213">
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="484.163" y1="442.196" x2="89.901" y2="60.229"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="101.376" y1="478.262" x2="443.18" y2="75.803"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="457.114" y1="126.623" x2="458.753" y2="363.508"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="541.54" y1="299.573" x2="543.179" y2="536.458"/>
</svg>`,
		},
		{
			name: "externalTest",
			input: `<?xml version="1.0" encoding="utf-8" ?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
<svg version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve">
<rect fill="url('http://example.com/benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
<rect fill="url('https://example.com/benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
<rect fill="  url(  ' https://example.com/benis.svg '  ) " x="0" y="0" width="1000" height="1000"></rect>
<rect fill="url('ftp://192.168.2.1/benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
<rect fill="url('//example.com/benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
<rect fill="url('/benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
<rect fill="url('#benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
</svg>`,
			/* Adjustement from https://github.com/darylldoyle/svg-sanitizer base
			--- Expected
			+++ Actual
			@@ -1,2 +1,2 @@
			-<?xml version="1.0" encoding="utf-8" ?>110
			-<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve">
			         +<svg version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve">
			@@ -6,4 +6,4 @@
			 <rect x="0" y="0" width="1000" height="1000"></rect>
			-<rect fill="url('/benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
			-<rect fill="url('#benis.svg')" x="0" y="0" width="1000" height="1000"></rect>
			+<rect x="0" y="0" width="1000" height="1000"></rect>
			+<rect x="0" y="0" width="1000" height="1000"></rect>
			FIXME: Currently this will block any fill with url.
			*/
			want: `<svg version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve">
<rect x="0" y="0" width="1000" height="1000"></rect>
<rect x="0" y="0" width="1000" height="1000"></rect>
<rect x="0" y="0" width="1000" height="1000"></rect>
<rect x="0" y="0" width="1000" height="1000"></rect>
<rect x="0" y="0" width="1000" height="1000"></rect>
<rect x="0" y="0" width="1000" height="1000"></rect>
<rect x="0" y="0" width="1000" height="1000"></rect>
</svg>`,
		},
		{
			name: "hrefTestOne",
			input: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" height="600px" id="Layer_1" width="600px" x="0px" y="0px" xml:space="preserve">
	<a href="javascript:alert(2)">test 1</a>
	<a xlink:href="javascript:alert(2)">test 2</a>
	<a href="#test3">test 3</a>
	<a xlink:href="#test">test 4</a>

	<a href="data:data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' onload='alert(88)'%3E%3C/svg%3E">test 5</a>
	<a xlink:href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' onload='alert(88)'%3E%3C/svg%3E">test 6</a>

	<a href="javascript&#9;:alert(document.domain)">test 7</a>
	<a href="javascrip&#9;t:alert('0xd0ff9')">test 8</a>
</svg>`,
			/* Adjustement from https://github.com/darylldoyle/svg-sanitizer base
			   Diff:
			   --- Expected
			   +++ Actual
			   @@ -5,6 +5,4 @@
			           <a xlink:href="#test">test 4</a>
			   -
			           <a>test 5</a>
			           <a>test 6</a>
			   -
			           <a>test 7</a>
			*/
			want: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" height="600px" id="Layer_1" width="600px" x="0px" y="0px" xml:space="preserve">
	<a>test 1</a>
	<a>test 2</a>
	<a href="#test3">test 3</a>
	<a xlink:href="#test">test 4</a>
	<a>test 5</a>
	<a>test 6</a>
	<a>test 7</a>
	<a>test 8</a>
</svg>`,
		},
		{
			name: "hrefTestTwo",
			input: `<svg xmlns="http://www.w3.org/2000/svg" height="600px" id="Layer_1" width="600px" x="0px" y="0px" xml:space="preserve">
	<a href="javascript:alert(2)">test 1</a>
	<a xlink:href="javascript:alert(2)">test 2</a>
	<a href="#test3">test 3</a>
	<a xlink:href="#test">test 4</a>

	<a href="data:data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' onload='alert(88)'%3E%3C/svg%3E">test 5</a>
	<a xlink:href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' onload='alert(88)'%3E%3C/svg%3E">test 6</a>

	<a href="javascript&#9;:alert(document.domain)">test 7</a>
	<a href="javascrip&#9;t:alert('0xd0ff9')">test 8</a>
</svg>`,
			/* Adjustement from https://github.com/darylldoyle/svg-sanitizer base
			   Diff:
			   --- Expected
			   +++ Actual
			   @@ -5,6 +5,4 @@
			           <a xlink:href="#test">test 4</a>
			   -
			           <a>test 5</a>
			           <a>test 6</a>
			   -
			           <a>test 7</a>
			*/
			want: `<svg xmlns="http://www.w3.org/2000/svg" height="600px" id="Layer_1" width="600px" x="0px" y="0px" xml:space="preserve">
	<a>test 1</a>
	<a>test 2</a>
	<a href="#test3">test 3</a>
	<a xlink:href="#test">test 4</a>
	<a>test 5</a>
	<a>test 6</a>
	<a>test 7</a>
	<a>test 8</a>
</svg>`,
		},
		{
			name: "svgTestOne",
			input: `<?xml version="1.0" encoding="utf-8"?>
<!-- Generator: Adobe Illustrator 16.0.0, SVG Export Plug-In . SVG Version: 6.00 Build 0)  -->
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
	 width="600px" height="600px" viewBox="0 0 600 600" enable-background="new 0 0 600 600" xml:space="preserve">
<line onload="alert(2)" fill="none" stroke="#000000" stroke-miterlimit="10" x1="119" y1="84.5" x2="454" y2="84.5"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="111.212" y1="102.852" x2="112.032" y2="476.623"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="198.917" y1="510.229" x2="486.622" y2="501.213"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="484.163" y1="442.196" x2="89.901" y2="60.229"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="101.376" y1="478.262" x2="443.18" y2="75.803"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="457.114" y1="126.623" x2="458.753" y2="363.508"/>
<this>shouldn't be here</this>
<script>alert(1);</script>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="541.54" y1="299.573" x2="543.179" y2="536.458"/>
</svg>`,
			/* Adjustement from https://github.com/darylldoyle/svg-sanitizer base
			   Diff:
			   --- Expected
			   +++ Actual
			   @@ -1,3 +1,2 @@
			   -<?xml version="1.0" encoding="utf-8"?>
			   -<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" id="Layer_1" version="1.1" x="0px" y="0px" width="600px" height="600px" viewBox="0 0 600 600" xml:space="preserve">
			   +<svg id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" width="600px" height="600px" viewbox="0 0 600 600" xml:space="preserve">
			    <line fill="none" stroke="#000000" stroke-miterlimit="10" x1="119" y1="84.5" x2="454" y2="84.5"/>
			   @@ -8,4 +7,2 @@
			    <line fill="none" stroke="#000000" stroke-miterlimit="10" x1="457.114" y1="126.623" x2="458.753" y2="363.508"/>
			   -
			   -
			    <line fill="none" stroke="#000000" stroke-miterlimit="10" x1="541.54" y1="299.573" x2="543.179" y2="536.458"/>
			*/
			want: `<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" width="600px" height="600px" viewbox="0 0 600 600" xml:space="preserve">
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="119" y1="84.5" x2="454" y2="84.5"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="111.212" y1="102.852" x2="112.032" y2="476.623"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="198.917" y1="510.229" x2="486.622" y2="501.213"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="484.163" y1="442.196" x2="89.901" y2="60.229"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="101.376" y1="478.262" x2="443.18" y2="75.803"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="457.114" y1="126.623" x2="458.753" y2="363.508"/>
<line fill="none" stroke="#000000" stroke-miterlimit="10" x1="541.54" y1="299.573" x2="543.179" y2="536.458"/>
</svg>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := SanitizeSVG(bytes.NewBufferString(tt.input))
			assert.Equal(t, tt.want, out, "The sanitized svg is not equal")
		})
	}
}

// minifySVG compact svg strings to ease testing (could maybe useful leter in package)
func minifySVG(svgData io.Reader) (*bytes.Buffer, error) {
	m := minify.New()
	m.AddFunc("image/svg+xml", svg.Minify)
	var out bytes.Buffer
	err := m.Minify("image/svg+xml", &out, svgData)
	return &out, err
}
