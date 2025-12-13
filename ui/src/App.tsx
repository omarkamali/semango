import { useState, useEffect } from 'react'
import { Search, Moon, Sun, FileText, Code, Image, Music, BookOpen } from 'lucide-react'
import { ApiDocs } from './components/ApiDocs'

interface SearchResult {
	rank: number
	score: number
	modality: string
	document: {
		path: string
		meta?: Record<string, string>
	}
	chunk: string
	highlights?: Record<string, unknown>
}

interface SearchResponse {
	results: SearchResult[]
	query: string
	top_k: number
	took: string
}

function App() {
	const [query, setQuery] = useState('')
	const [results, setResults] = useState<SearchResult[]>([])
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState<string | null>(null)
	const [darkMode, setDarkMode] = useState(false)
	const [searchTime, setSearchTime] = useState<string>('')
	const [showApiDocs, setShowApiDocs] = useState(false)

	console.log({ results })
	// Initialize dark mode from localStorage
	useEffect(() => {
		const savedDarkMode = localStorage.getItem('darkMode') === 'true'
		setDarkMode(savedDarkMode)
		if (savedDarkMode) {
			document.documentElement.classList.add('dark')
		}
	}, [])

	// Toggle dark mode
	const toggleDarkMode = () => {
		const newDarkMode = !darkMode
		setDarkMode(newDarkMode)
		localStorage.setItem('darkMode', newDarkMode.toString())
		if (newDarkMode) {
			document.documentElement.classList.add('dark')
		} else {
			document.documentElement.classList.remove('dark')
		}
	}

	// Toggle API Docs view
	const toggleApiDocs = () => {
		setShowApiDocs((prev) => !prev)
	}

	// Perform search
	const handleSearch = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!query.trim()) return

		setLoading(true)
		setError(null)
		setShowApiDocs(false)

		try {
			const response = await fetch('/api/v1/search', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					query: query.trim(),
					top_k: 10,
				}),
			})

			if (!response.ok) {
				throw new Error(`Search failed: ${response.statusText}`)
			}

			const data: SearchResponse = await response.json()
			setResults(data.results)
			setSearchTime(data.took)
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Search failed')
			setResults([])
		} finally {
			setLoading(false)
		}
	}

	// Get icon for modality
	const getModalityIcon = (modality: string) => {
		switch (modality.toLowerCase()) {
			case 'text':
				return <FileText className="w-4 h-4" />
			case 'code':
				return <Code className="w-4 h-4" />
			case 'image':
				return <Image className="w-4 h-4" />
			case 'audio':
				return <Music className="w-4 h-4" />
			default:
				return <FileText className="w-4 h-4" />
		}
	}

	// Format score as percentage
	const formatScore = (score: number) => {
		return `${(score * 100).toFixed(1)}%`
	}

	return (
		<div className="min-h-screen bg-background text-foreground">
			{/* Header */}
			<header className="border-b border-border">
				<div className="container mx-auto px-4 py-4 flex items-center justify-between">
					<div className="flex items-center space-x-4">
						<span className="text-2xl font-bold">🥭</span>
						<h1 className="text-xl font-bold">Semango</h1>
					</div>
					<div className="flex items-center space-x-2">
						<button
							onClick={toggleApiDocs}
							className="p-2 rounded-md hover:bg-accent hover:text-accent-foreground transition-colors"
							aria-label="Toggle API Documentation"
						>
							<BookOpen className="w-5 h-5" />
						</button>
						<button
							onClick={toggleDarkMode}
							className="p-2 rounded-md hover:bg-accent hover:text-accent-foreground transition-colors"
							aria-label="Toggle dark mode"
						>
							{darkMode ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
						</button>
					</div>
				</div>
			</header>

			{/* Main Content */}
			<main className="container mx-auto px-4 py-8">
				{/* Search Form (always visible unless API docs are shown) */}
				{!showApiDocs && (
					<form onSubmit={handleSearch} className="mb-8">
						<div className="relative max-w-2xl mx-auto">
							<Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground w-5 h-5" />
							<input
								type="text"
								value={query}
								onChange={(e) => setQuery(e.target.value)}
								placeholder="Search your knowledge base..."
								className="w-full pl-10 pr-4 py-3 text-lg border border-input rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent"
								disabled={loading}
							/>
							<button
								type="submit"
								disabled={loading || !query.trim()}
								className="absolute right-2 top-1/2 transform -translate-y-1/2 px-4 py-1.5 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{loading ? 'Searching...' : 'Search'}
							</button>
						</div>
					</form>
				)}

				{/* API Docs */}
				{showApiDocs && <ApiDocs />}

				{/* Search Results (only show if not showing API docs) */}
				{!showApiDocs && error && (
					<div className="max-w-4xl mx-auto mb-6 p-4 bg-destructive/10 border border-destructive/20 rounded-lg">
						<p className="text-destructive">{error}</p>
					</div>
				)}

				{!showApiDocs && results.length > 0 && (
					<div className="max-w-4xl mx-auto">
						<div className="mb-4 text-sm text-muted-foreground">
							Found {results.length} results in {searchTime}
						</div>

						<div className="space-y-4">
							{results.map((result) => (
								<div
									key={`${result.document.path}-${result.rank}`}
									className="p-6 bg-card border border-border rounded-lg hover:shadow-md transition-shadow"
								>
									{/* Result Header */}
									<div className="flex items-center justify-between mb-3">
										<div className="flex items-center space-x-2">
											{getModalityIcon(result.modality)}
											<span className="text-sm font-medium text-muted-foreground capitalize">
												{result.modality}
											</span>
											<span className="text-sm text-muted-foreground">•</span>
											<span className="text-sm font-medium text-primary">
												{formatScore(result.score)}
											</span>
										</div>
										<span className="text-xs text-muted-foreground">
											#{result.rank}
										</span>
									</div>

									{/* Document Path */}
									<div className="mb-3">
										<code className="text-sm bg-muted px-2 py-1 rounded font-mono">
											{result.document.path}
										</code>
									</div>

									{/* Content Preview */}
									<div className="mb-3">
										<p className="text-sm leading-relaxed">
											{result.chunk}
										</p>
									</div>

									{/* Metadata */}
									{result.document.meta && Object.keys(result.document.meta).length > 0 && (
										<div className="flex flex-wrap gap-2">
											{Object.entries(result.document.meta).map(([key, value]) => (
												<span
													key={key}
													className="text-xs bg-secondary text-secondary-foreground px-2 py-1 rounded"
												>
													{key}: {value}
												</span>
											))}
										</div>
									)}
								</div>
							))}
						</div>
					</div>
				)}

				{/* Empty State / Welcome State / API Docs */}
				{!showApiDocs && !query && results.length === 0 && (
					<div className="max-w-2xl mx-auto text-center py-12">
						<div className="text-4xl font-bold mb-4">🥭</div>
						<h2 className="text-2xl font-bold mb-4">Welcome to Semango Search</h2>
						<p className="text-muted-foreground mb-8">
							Search through your knowledge base using semantic search.
							Find documents, code, images, and more with natural language queries.
						</p>
						<div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-left">
							<div className="p-4 bg-card border border-border rounded-lg">
								<h3 className="font-medium mb-2">Example Searches</h3>
								<ul className="text-sm text-muted-foreground space-y-1">
									<li>• "payment retry logic"</li>
									<li>• "authentication middleware"</li>
									<li>• "database connection setup"</li>
								</ul>
							</div>
							<div className="p-4 bg-card border border-border rounded-lg">
								<h3 className="font-medium mb-2">API Access</h3>
								<p className="text-sm text-muted-foreground">
									You can also interact with Semango via its REST API.
									Click the <BookOpen className="inline-block w-4 h-4" /> icon in the header to view API documentation.
								</p>
							</div>
						</div>
					</div>
				)}
			</main>
		</div>
	)
}

export default App 