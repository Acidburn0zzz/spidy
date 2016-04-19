package spidy_test

var validPaths = map[string]bool{
	"/bootstrap/css/bootstrap.css":                             true,
	"/assets/application-4b77637cc302ef4af6c358864df26f88.css": true,
	"https://www.youtube.com/player_api":                       true,
	"/assets/application-9709f2e1ad6d5ec24402f59507f6822b.js":  true,
	"/assets/application-valum.js.js":                          true,
	"/services":                                                true,
	"/contacts":                                                true,
	"/assets/ardan-symbol-93ee488d16f9bc56ad65659c2d8f41dc.png": true,
	"/assets/member1-55a2b7ac0a868d49fdf50ce39f0ce1ac.png":      true,
	"/assets/member2-66485427ca4bd140e0547efb1ce12ce0.png":      true,
	"/assets/member4-cfa03a1a15aed816528b8ec1ee6c95c6.png":      true,
	"/assets/member5-6ee6a979c39c81e2b652f268cccaf265.png":      true,
}

// ardan provides a buffer for that defines good links for testing.
var ardan = []byte(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
	<title>Ardan Studios</title>
</head>
<body>

		<link rel="stylesheet" href="/bootstrap/css/bootstrap.css">
		<link href="/assets/application-4b77637cc302ef4af6c358864df26f88.css" media="screen" rel="stylesheet" />

		<script src="https://www.youtube.com/player_api"></script>
		<script src="/assets/application-9709f2e1ad6d5ec24402f59507f6822b.js"></script>
		<script src="/assets/application-valum.js.js"></script>

		<a href="/services"></a>
		<a href="/contacts"></a>

		<a href="http://youtube.com/x8433j4i"></a>
		<a href="http://gracehound.com/index"></a>

		<img class="ardan-symbol" src="/assets/ardan-symbol-93ee488d16f9bc56ad65659c2d8f41dc.png" />
		<img src="/assets/member1-55a2b7ac0a868d49fdf50ce39f0ce1ac.png" />
		<img src="/assets/member2-66485427ca4bd140e0547efb1ce12ce0.png" />
		<img src="/assets/member4-cfa03a1a15aed816528b8ec1ee6c95c6.png" />
		<img src="/assets/member5-6ee6a979c39c81e2b652f268cccaf265.png" />

</body>
</html>`)

// ardanBadImages provides a buffer for that defines bad image links for testing.
var ardanBadImages = []byte(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
	<title>Ardan Studios</title>
<body>

		<img class="ardan-symbol" src="/assets/ardan-symbol-93ee488d16f9bc56ad65659c2d8f41dc.png" />
		<img src="/assets/member1-55a2b7ac0a868d49fdf50ce39f0ce1ac.png" />
		<img src="/assets/member2-66485427ca4bd140e0547efb1ce12ce0.png" />
		<img src="/assets/member4-cfa03a1a15aed816528b8ec1ee6c95c6.png" />
		<img src="/assets/member5-6ee6a979c39c81e2b652f268cccaf265.png" />
		<img src="/assets/member6-e202d0df26e17043328648feda1fc327.png" />
		<img src="/assets/member7-cbcbc8bfe0d8f0cefe66a1b801827f74.png" />
		<img src="/assets/member10-01462a64b08492ba3b64058ea50b94f8.png" />

</body>
</html>`)

// ardanBadLink provides a buffer of bad links within the ardanlabs page.
var ardanBadLink = []byte(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Ardan Studios</title>
</head>
<body>

		<link rel="stylesheet" href="/bootstrap/css/bootstrap.css">
		<link href="/maxcdn.bootstrapcdn.com/font-awesome/4.2.0/css/font-awesome.min.css" type="text/css" rel="stylesheet" />
		<link href="/assets/application-4b77637cc302ef4af6c358864df26f88.css" media="screen" rel="stylesheet" />

		<a href="/services"></a>
		<a href="/contacts"></a>
		<a href="/billy"></a>
		<a href="/wacksee"></a>

		<a href="http://youtube.com/x8433j4i"></a>
		<a href="http://gracehound.com/index"></a>

</body>
</html>`)

// ardanBadScripts provides a buffer for that defines bad script links for testing.
var ardanBadScripts = []byte(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
	<title>Ardan Studios</title>
</head>
<body>
	<script src="/assets/application-9709f2e1ad6d5ec24402f59507f6822b.js"></script>
	<script src="/assets/application-blacksmith.js"></script>
	<script src="/assets/application-trottle.js.js"></script>
	<script src="/assets/application-valum.js.js"></script>
	<script src="https://www.youtube.com/player_api"></script>
</body>
</html>`)
