jQuery.fn.autolink = function() {
	var re = /((([A-Za-z]{3,9}:(?:\/\/)?)(?:[\-;:&=\+\$,\w]+@)?[A-Za-z0-9\.\-]+|(?:www\.|[\-;:&=\+\$,\w]+@)[A-Za-z0-9\.\-]+)((?:\/[\+~%\/\.\w\-]*)?\??(?:[\-\+:=&;%@\.\w]*)#?(?:[\.\!\/\\\w]*))?)/g;
	return this.find('*').contents()
		.filter(function () { return this.nodeType === 3; })
		.each(function() {
			$(this).each(function() {
				if (re.test($(this).text()))
					$(this).replaceWith(
						$("<span />").html(
							this.nodeValue.replace(re, "<a href='$1'>$1</a>")
						)
					);
			});
		});
};