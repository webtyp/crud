module webtyp.com/crud

go 1.26.8

require (
	webtyp.com/fmt v1.0.0
	webtyp.com/json v0.5.40
	webtyp.com/model v0.2.13
	webtyp.com/router v0.4.17
)

require webtyp.com/escape v0.1.0 // indirect

replace webtyp.com/model => ../model

replace webtyp.com/router => ../router
