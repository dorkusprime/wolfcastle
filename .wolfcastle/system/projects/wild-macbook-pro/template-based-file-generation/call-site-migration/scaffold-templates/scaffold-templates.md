# Scaffold Templates

Create .tmpl files for scaffold-generated files (gitignore, five READMEs) and migrate scaffold_service.go to use RenderToFile. These are static templates with no variables. The scaffoldREADMEs map and inline gitignore string in scaffold_service.go are replaced with template files under templates/scaffold/.
