package plugin

const (
	defaultTemplate = `
%use square brackets as golang text templating delimiters
\documentclass{article}
\usepackage{graphicx}
[[- if .IsLandscapeOrientation]] 
	\usepackage[landscape,margin=0.1in]{geometry} 
[[- else]] 
	\usepackage[margin=0.1in]{geometry} 
[[- end]] 

\graphicspath{ {images/} }
\begin{document}

\title{
	[[.Title]] 
	[[- if .VariableValues]] 
		\\ \large [[.VariableValues]] 
	[[- end]] 
	[[- if .Description]] 
		\\ \small [[.Description]] 
	[[- end]]
}

\date{
	[[.From]]\\
	to\\
	[[.To]]
}

\maketitle

\begin{center}

[[- if .IsGridLayout]]
	[[- range .Panels]]
		[[- if .IsPartialWidth]]
			\begin{minipage}{[[.Width]]\textwidth}
			\includegraphics[width=\textwidth]{image[[.Id]]}
			\end{minipage}
		[[- else]]\par
			\vspace{0.5cm}
			\includegraphics[width=\textwidth]{image[[.Id]]}
			\par
			\vspace{0.5cm}
		[[- end]]
		%
	[[- end]]
[[else]]
	[[- range .Panels]]
		[[- if .IsSingleStat]]
			\begin{minipage}{0.3\textwidth}
			\includegraphics[width=\textwidth]{image[[.Id]]}
			\end{minipage}
		[[- else]]\par
			\vspace{0.5cm}
			\includegraphics[width=\textwidth]{image[[.Id]]}
			\par
			\vspace{0.5cm}
		[[- end]]
	[[- end]]
[[- end]]

\end{center}
\end{document}
`
)
