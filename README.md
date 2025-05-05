# mega-sistema-backend-pocketbase
 
 ## Objetivo
 El presente proyecto es un microservicio para la materia de Tecnologias Web 2 y es parte de un proyecto mas grande como un Sistema integrador de la carrera. En este proyecto mas grande se identifico como necesario tener un microservicio para la generacion de documentos para asi tener el modulo separado y abrierto a ser flexible a nuevas disposiciones de ser necesario y facilitar la creacion de documentacion como ser: licencias y certificados de manera automatizada
 ## Implementacion
 ```
npm install
go tidy
go run . serve
```
 ## Datos esperados
### Licencias
Se expone dos endpoint unos para mostrar la licencia y otro para descargarla:
```
POST /mostrar-licencia
POST /descargar-licencia
```
Estos dos endpoint reciben la misma información desde un JSON con los datos a incluir en el documento. La respuesta contiene un enlace para visualizar o descargar el archivo PDF generado.
Los datos en el json son:
```
Nombre del estudiante: "nombre_estudiante"
Nombre del director: "nombre_director"
Fecha inicio de la licencia: "fecha_inicio_licencia"
Fecha fin de la licencia: "fecha_fin_licencia"
```
### Certificados
Se tiene un endpoint con el nombre de la platilla que se quiera usar para el certificado
```
POST /mostrar-certificados? filename = nombrePlantilla
```
Se recibe un JSON con los datos a incluir en el documento y con respuesta el PDF para visualizar. Los datos en el json son:
```
Nombre del estudiante: "nombre_estudiante"
Nombre del director: "nombre_ director "
Nombre del certificado: "nombre_ certificado "
```
