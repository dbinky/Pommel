package chunker

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions
// =============================================================================

func createJavaScriptSourceFile(path string, content string, lang string) *models.SourceFile {
	return &models.SourceFile{
		Path:         path,
		Content:      []byte(content),
		Language:     lang,
		LastModified: time.Now(),
	}
}

func findChunkByName(chunks []*models.Chunk, name string) *models.Chunk {
	for _, chunk := range chunks {
		if chunk.Name == name {
			return chunk
		}
	}
	return nil
}

func findChunksByLevel(chunks []*models.Chunk, level models.ChunkLevel) []*models.Chunk {
	var result []*models.Chunk
	for _, chunk := range chunks {
		if chunk.Level == level {
			result = append(result, chunk)
		}
	}
	return result
}

// =============================================================================
// JavaScriptChunker Initialization Tests
// =============================================================================

func TestNewJavaScriptChunker(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)
	assert.NotNil(t, chunker, "NewJavaScriptChunker should return a non-nil chunker")
}

func TestNewJavaScriptChunker_AllLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	languages := []Language{
		LangJavaScript,
		LangTypeScript,
		LangJSX,
		LangTSX,
	}

	for _, lang := range languages {
		t.Run(string(lang), func(t *testing.T) {
			chunker := NewJavaScriptChunker(parser, lang)
			assert.NotNil(t, chunker)
			assert.Equal(t, lang, chunker.Language())
		})
	}
}

// =============================================================================
// ES6 Class Tests
// =============================================================================

func TestJavaScriptChunker_ES6Class_Basic(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }

    subtract(a, b) {
        return a - b;
    }
}`

	file := createJavaScriptSourceFile("/test/calculator.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 1 class chunk + 2 method chunks = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have file, class, and method chunks")

	// Verify file chunk exists
	fileChunks := findChunksByLevel(result.Chunks, models.ChunkLevelFile)
	assert.Len(t, fileChunks, 1, "Should have exactly one file chunk")

	// Verify class chunk exists
	classChunks := findChunksByLevel(result.Chunks, models.ChunkLevelClass)
	assert.Len(t, classChunks, 1, "Should have exactly one class chunk")
	assert.Equal(t, "Calculator", classChunks[0].Name)

	// Verify method chunks exist
	methodChunks := findChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.Len(t, methodChunks, 2, "Should have two method chunks")

	addMethod := findChunkByName(result.Chunks, "add")
	assert.NotNil(t, addMethod, "Should find 'add' method")
	assert.Equal(t, models.ChunkLevelMethod, addMethod.Level)

	subtractMethod := findChunkByName(result.Chunks, "subtract")
	assert.NotNil(t, subtractMethod, "Should find 'subtract' method")
	assert.Equal(t, models.ChunkLevelMethod, subtractMethod.Level)
}

func TestJavaScriptChunker_ES6Class_WithConstructor(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Person {
    constructor(name, age) {
        this.name = name;
        this.age = age;
    }

    greet() {
        return "Hello, I'm " + this.name;
    }
}`

	file := createJavaScriptSourceFile("/test/person.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file + 1 class + 2 methods (constructor + greet)
	methodChunks := findChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.Len(t, methodChunks, 2, "Should have constructor and greet methods")

	constructor := findChunkByName(result.Chunks, "constructor")
	assert.NotNil(t, constructor, "Should find constructor method")
}

func TestJavaScriptChunker_ES6Class_StaticMethods(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class MathUtils {
    static add(a, b) {
        return a + b;
    }

    static multiply(a, b) {
        return a * b;
    }
}`

	file := createJavaScriptSourceFile("/test/math.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	methodChunks := findChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.Len(t, methodChunks, 2, "Should have two static method chunks")

	addMethod := findChunkByName(result.Chunks, "add")
	assert.NotNil(t, addMethod, "Should find static 'add' method")

	multiplyMethod := findChunkByName(result.Chunks, "multiply")
	assert.NotNil(t, multiplyMethod, "Should find static 'multiply' method")
}

func TestJavaScriptChunker_ES6Class_AsyncMethods(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class ApiClient {
    async fetchData(url) {
        const response = await fetch(url);
        return response.json();
    }

    async postData(url, data) {
        const response = await fetch(url, { method: 'POST', body: data });
        return response.json();
    }
}`

	file := createJavaScriptSourceFile("/test/api.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	methodChunks := findChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.Len(t, methodChunks, 2, "Should have two async method chunks")

	fetchMethod := findChunkByName(result.Chunks, "fetchData")
	assert.NotNil(t, fetchMethod, "Should find async 'fetchData' method")

	postMethod := findChunkByName(result.Chunks, "postData")
	assert.NotNil(t, postMethod, "Should find async 'postData' method")
}

func TestJavaScriptChunker_ES6Class_GettersSetters(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Rectangle {
    constructor(width, height) {
        this._width = width;
        this._height = height;
    }

    get area() {
        return this._width * this._height;
    }

    set width(value) {
        this._width = value;
    }
}`

	file := createJavaScriptSourceFile("/test/rectangle.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should find constructor, getter, and setter as methods
	methodChunks := findChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.GreaterOrEqual(t, len(methodChunks), 3, "Should have at least constructor, getter, and setter")
}

// =============================================================================
// Function Declaration Tests
// =============================================================================

func TestJavaScriptChunker_FunctionDeclaration(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function greet(name) {
    return "Hello, " + name;
}

function farewell(name) {
    return "Goodbye, " + name;
}`

	file := createJavaScriptSourceFile("/test/functions.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Top-level functions should be at method level
	methodChunks := findChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.Len(t, methodChunks, 2, "Should have two function chunks at method level")

	greetFunc := findChunkByName(result.Chunks, "greet")
	assert.NotNil(t, greetFunc, "Should find 'greet' function")
	assert.Equal(t, models.ChunkLevelMethod, greetFunc.Level)

	farewellFunc := findChunkByName(result.Chunks, "farewell")
	assert.NotNil(t, farewellFunc, "Should find 'farewell' function")
	assert.Equal(t, models.ChunkLevelMethod, farewellFunc.Level)
}

func TestJavaScriptChunker_AsyncFunctionDeclaration(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `async function fetchUser(id) {
    const response = await fetch('/users/' + id);
    return response.json();
}`

	file := createJavaScriptSourceFile("/test/async.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	fetchFunc := findChunkByName(result.Chunks, "fetchUser")
	assert.NotNil(t, fetchFunc, "Should find async 'fetchUser' function")
	assert.Equal(t, models.ChunkLevelMethod, fetchFunc.Level)
}

func TestJavaScriptChunker_GeneratorFunction(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function* numberGenerator() {
    yield 1;
    yield 2;
    yield 3;
}`

	file := createJavaScriptSourceFile("/test/generator.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	genFunc := findChunkByName(result.Chunks, "numberGenerator")
	assert.NotNil(t, genFunc, "Should find generator function")
	assert.Equal(t, models.ChunkLevelMethod, genFunc.Level)
}

// =============================================================================
// Arrow Function Tests
// =============================================================================

func TestJavaScriptChunker_ArrowFunction_Const(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `const greet = (name) => {
    return "Hello, " + name;
};

const farewell = (name) => {
    return "Goodbye, " + name;
};`

	file := createJavaScriptSourceFile("/test/arrows.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Arrow functions assigned to const should be extracted
	greetFunc := findChunkByName(result.Chunks, "greet")
	assert.NotNil(t, greetFunc, "Should find arrow function 'greet'")
	assert.Equal(t, models.ChunkLevelMethod, greetFunc.Level)

	farewellFunc := findChunkByName(result.Chunks, "farewell")
	assert.NotNil(t, farewellFunc, "Should find arrow function 'farewell'")
	assert.Equal(t, models.ChunkLevelMethod, farewellFunc.Level)
}

func TestJavaScriptChunker_ArrowFunction_Let(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `let add = (a, b) => a + b;

let multiply = (a, b) => {
    return a * b;
};`

	file := createJavaScriptSourceFile("/test/arrows_let.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	addFunc := findChunkByName(result.Chunks, "add")
	assert.NotNil(t, addFunc, "Should find arrow function 'add' with let")

	multiplyFunc := findChunkByName(result.Chunks, "multiply")
	assert.NotNil(t, multiplyFunc, "Should find arrow function 'multiply' with let")
}

func TestJavaScriptChunker_ArrowFunction_Async(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `const fetchData = async (url) => {
    const response = await fetch(url);
    return response.json();
};`

	file := createJavaScriptSourceFile("/test/async_arrow.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	fetchFunc := findChunkByName(result.Chunks, "fetchData")
	assert.NotNil(t, fetchFunc, "Should find async arrow function 'fetchData'")
	assert.Equal(t, models.ChunkLevelMethod, fetchFunc.Level)
}

// =============================================================================
// TypeScript Interface Tests
// =============================================================================

func TestJavaScriptChunker_TypeScriptInterface_Basic(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `interface User {
    id: number;
    name: string;
    email: string;
}`

	file := createJavaScriptSourceFile("/test/user.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Interfaces should be at class level
	classChunks := findChunksByLevel(result.Chunks, models.ChunkLevelClass)
	assert.Len(t, classChunks, 1, "Should have one interface chunk at class level")

	userInterface := findChunkByName(result.Chunks, "User")
	assert.NotNil(t, userInterface, "Should find 'User' interface")
	assert.Equal(t, models.ChunkLevelClass, userInterface.Level)
}

func TestJavaScriptChunker_TypeScriptInterface_WithMethods(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `interface Repository<T> {
    findById(id: string): T | null;
    findAll(): T[];
    save(entity: T): void;
    delete(id: string): boolean;
}`

	file := createJavaScriptSourceFile("/test/repository.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	repoInterface := findChunkByName(result.Chunks, "Repository")
	assert.NotNil(t, repoInterface, "Should find 'Repository' interface")
	assert.Equal(t, models.ChunkLevelClass, repoInterface.Level)
}

func TestJavaScriptChunker_TypeScriptInterface_Extends(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `interface Animal {
    name: string;
}

interface Dog extends Animal {
    breed: string;
    bark(): void;
}`

	file := createJavaScriptSourceFile("/test/animals.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	classChunks := findChunksByLevel(result.Chunks, models.ChunkLevelClass)
	assert.Len(t, classChunks, 2, "Should have two interface chunks")

	animalInterface := findChunkByName(result.Chunks, "Animal")
	assert.NotNil(t, animalInterface, "Should find 'Animal' interface")

	dogInterface := findChunkByName(result.Chunks, "Dog")
	assert.NotNil(t, dogInterface, "Should find 'Dog' interface")
}

// =============================================================================
// TypeScript Type Alias Tests
// =============================================================================

func TestJavaScriptChunker_TypeScriptTypeAlias(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `type UserId = string;

type UserRole = 'admin' | 'user' | 'guest';

type UserState = {
    isLoggedIn: boolean;
    lastLogin: Date;
};`

	file := createJavaScriptSourceFile("/test/types.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Type aliases should be at class level
	classChunks := findChunksByLevel(result.Chunks, models.ChunkLevelClass)
	assert.GreaterOrEqual(t, len(classChunks), 1, "Should have type alias chunks")
}

// =============================================================================
// Export Tests
// =============================================================================

func TestJavaScriptChunker_ExportedClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `export class Calculator {
    add(a, b) {
        return a + b;
    }
}`

	file := createJavaScriptSourceFile("/test/exported.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	calcClass := findChunkByName(result.Chunks, "Calculator")
	assert.NotNil(t, calcClass, "Should find exported 'Calculator' class")
	assert.Equal(t, models.ChunkLevelClass, calcClass.Level)

	addMethod := findChunkByName(result.Chunks, "add")
	assert.NotNil(t, addMethod, "Should find 'add' method")
}

func TestJavaScriptChunker_ExportDefaultClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `export default class MainService {
    initialize() {
        console.log("Initialized");
    }
}`

	file := createJavaScriptSourceFile("/test/default.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	serviceClass := findChunkByName(result.Chunks, "MainService")
	assert.NotNil(t, serviceClass, "Should find default exported 'MainService' class")
}

func TestJavaScriptChunker_ExportedFunction(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `export function calculateTotal(items) {
    return items.reduce((sum, item) => sum + item.price, 0);
}

export const formatCurrency = (amount) => {
    return '$' + amount.toFixed(2);
};`

	file := createJavaScriptSourceFile("/test/exports.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	calcFunc := findChunkByName(result.Chunks, "calculateTotal")
	assert.NotNil(t, calcFunc, "Should find exported 'calculateTotal' function")

	formatFunc := findChunkByName(result.Chunks, "formatCurrency")
	assert.NotNil(t, formatFunc, "Should find exported 'formatCurrency' arrow function")
}

func TestJavaScriptChunker_TypeScriptExportedInterface(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `export interface Config {
    apiUrl: string;
    timeout: number;
    retries: number;
}

export interface Logger {
    log(message: string): void;
    error(message: string): void;
}`

	file := createJavaScriptSourceFile("/test/exported_interfaces.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	configInterface := findChunkByName(result.Chunks, "Config")
	assert.NotNil(t, configInterface, "Should find exported 'Config' interface")

	loggerInterface := findChunkByName(result.Chunks, "Logger")
	assert.NotNil(t, loggerInterface, "Should find exported 'Logger' interface")
}

// =============================================================================
// Parent-Child Relationship Tests
// =============================================================================

func TestJavaScriptChunker_ParentChild_MethodsReferenceClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }

    subtract(a, b) {
        return a - b;
    }
}`

	file := createJavaScriptSourceFile("/test/parent_child.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find the class chunk
	calcClass := findChunkByName(result.Chunks, "Calculator")
	require.NotNil(t, calcClass, "Should find Calculator class")

	// Find method chunks and verify they reference the class
	addMethod := findChunkByName(result.Chunks, "add")
	require.NotNil(t, addMethod, "Should find add method")
	assert.NotNil(t, addMethod.ParentID, "Method should have a parent ID")
	assert.Equal(t, calcClass.ID, *addMethod.ParentID, "Method's parent should be the class")

	subtractMethod := findChunkByName(result.Chunks, "subtract")
	require.NotNil(t, subtractMethod, "Should find subtract method")
	assert.NotNil(t, subtractMethod.ParentID, "Method should have a parent ID")
	assert.Equal(t, calcClass.ID, *subtractMethod.ParentID, "Method's parent should be the class")
}

func TestJavaScriptChunker_ParentChild_ClassReferencesFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }
}`

	file := createJavaScriptSourceFile("/test/file_parent.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find the file chunk
	fileChunks := findChunksByLevel(result.Chunks, models.ChunkLevelFile)
	require.Len(t, fileChunks, 1, "Should have one file chunk")
	fileChunk := fileChunks[0]

	// Find the class chunk and verify it references the file
	calcClass := findChunkByName(result.Chunks, "Calculator")
	require.NotNil(t, calcClass, "Should find Calculator class")
	assert.NotNil(t, calcClass.ParentID, "Class should have a parent ID")
	assert.Equal(t, fileChunk.ID, *calcClass.ParentID, "Class's parent should be the file")
}

func TestJavaScriptChunker_ParentChild_TopLevelFunctionReferencesFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function greet(name) {
    return "Hello, " + name;
}`

	file := createJavaScriptSourceFile("/test/top_level.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find the file chunk
	fileChunks := findChunksByLevel(result.Chunks, models.ChunkLevelFile)
	require.Len(t, fileChunks, 1, "Should have one file chunk")
	fileChunk := fileChunks[0]

	// Top-level functions should reference the file as parent
	greetFunc := findChunkByName(result.Chunks, "greet")
	require.NotNil(t, greetFunc, "Should find greet function")
	assert.NotNil(t, greetFunc.ParentID, "Top-level function should have a parent ID")
	assert.Equal(t, fileChunk.ID, *greetFunc.ParentID, "Top-level function's parent should be the file")
}

// =============================================================================
// JSX Tests
// =============================================================================

func TestJavaScriptChunker_JSX_FunctionComponent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJSX)

	source := `function Greeting({ name }) {
    return <div>Hello, {name}!</div>;
}

function Farewell({ name }) {
    return <div>Goodbye, {name}!</div>;
}`

	file := createJavaScriptSourceFile("/test/components.jsx", source, "jsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	greetingComponent := findChunkByName(result.Chunks, "Greeting")
	assert.NotNil(t, greetingComponent, "Should find 'Greeting' component")
	assert.Equal(t, models.ChunkLevelMethod, greetingComponent.Level)

	farewellComponent := findChunkByName(result.Chunks, "Farewell")
	assert.NotNil(t, farewellComponent, "Should find 'Farewell' component")
}

func TestJavaScriptChunker_JSX_ArrowComponent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJSX)

	source := `const Button = ({ onClick, children }) => {
    return <button onClick={onClick}>{children}</button>;
};

const Card = ({ title, content }) => (
    <div className="card">
        <h2>{title}</h2>
        <p>{content}</p>
    </div>
);`

	file := createJavaScriptSourceFile("/test/arrow_components.jsx", source, "jsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	buttonComponent := findChunkByName(result.Chunks, "Button")
	assert.NotNil(t, buttonComponent, "Should find 'Button' arrow component")

	cardComponent := findChunkByName(result.Chunks, "Card")
	assert.NotNil(t, cardComponent, "Should find 'Card' arrow component")
}

func TestJavaScriptChunker_JSX_ClassComponent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJSX)

	source := `class Counter extends React.Component {
    constructor(props) {
        super(props);
        this.state = { count: 0 };
    }

    increment() {
        this.setState({ count: this.state.count + 1 });
    }

    render() {
        return <div>{this.state.count}</div>;
    }
}`

	file := createJavaScriptSourceFile("/test/class_component.jsx", source, "jsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counterClass := findChunkByName(result.Chunks, "Counter")
	assert.NotNil(t, counterClass, "Should find 'Counter' class component")
	assert.Equal(t, models.ChunkLevelClass, counterClass.Level)

	// Should find methods
	renderMethod := findChunkByName(result.Chunks, "render")
	assert.NotNil(t, renderMethod, "Should find 'render' method")

	incrementMethod := findChunkByName(result.Chunks, "increment")
	assert.NotNil(t, incrementMethod, "Should find 'increment' method")
}

// =============================================================================
// TSX Tests
// =============================================================================

func TestJavaScriptChunker_TSX_FunctionComponent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTSX)

	source := `interface Props {
    name: string;
}

function Greeting({ name }: Props): JSX.Element {
    return <div>Hello, {name}!</div>;
}`

	file := createJavaScriptSourceFile("/test/typed_component.tsx", source, "tsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	propsInterface := findChunkByName(result.Chunks, "Props")
	assert.NotNil(t, propsInterface, "Should find 'Props' interface")
	assert.Equal(t, models.ChunkLevelClass, propsInterface.Level)

	greetingComponent := findChunkByName(result.Chunks, "Greeting")
	assert.NotNil(t, greetingComponent, "Should find 'Greeting' component")
}

func TestJavaScriptChunker_TSX_ArrowComponentWithTypes(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTSX)

	source := `interface ButtonProps {
    onClick: () => void;
    disabled?: boolean;
    children: React.ReactNode;
}

const Button: React.FC<ButtonProps> = ({ onClick, disabled, children }) => {
    return (
        <button onClick={onClick} disabled={disabled}>
            {children}
        </button>
    );
};`

	file := createJavaScriptSourceFile("/test/typed_arrow.tsx", source, "tsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	buttonPropsInterface := findChunkByName(result.Chunks, "ButtonProps")
	assert.NotNil(t, buttonPropsInterface, "Should find 'ButtonProps' interface")

	buttonComponent := findChunkByName(result.Chunks, "Button")
	assert.NotNil(t, buttonComponent, "Should find 'Button' typed arrow component")
}

// =============================================================================
// Multiple Languages Tests
// =============================================================================

func TestJavaScriptChunker_JavaScript(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)
	assert.Equal(t, LangJavaScript, chunker.Language())

	source := `class Test { method() { } }`
	file := createJavaScriptSourceFile("/test/test.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestJavaScriptChunker_TypeScript(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)
	assert.Equal(t, LangTypeScript, chunker.Language())

	source := `interface Test { id: number; }`
	file := createJavaScriptSourceFile("/test/test.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestJavaScriptChunker_JSX(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJSX)
	assert.Equal(t, LangJSX, chunker.Language())

	source := `function Component() { return <div>Hello</div>; }`
	file := createJavaScriptSourceFile("/test/test.jsx", source, "jsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestJavaScriptChunker_TSX(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTSX)
	assert.Equal(t, LangTSX, chunker.Language())

	source := `function Component(): JSX.Element { return <div>Hello</div>; }`
	file := createJavaScriptSourceFile("/test/test.tsx", source, "tsx")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// =============================================================================
// Chunk Properties Tests
// =============================================================================

func TestJavaScriptChunker_ChunkProperties_LineNumbers(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }
}`

	file := createJavaScriptSourceFile("/test/lines.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify chunks have proper line numbers
	for _, chunk := range result.Chunks {
		assert.GreaterOrEqual(t, chunk.StartLine, 1, "StartLine should be >= 1")
		assert.GreaterOrEqual(t, chunk.EndLine, chunk.StartLine, "EndLine should be >= StartLine")
	}

	// File chunk should span entire file
	fileChunks := findChunksByLevel(result.Chunks, models.ChunkLevelFile)
	require.Len(t, fileChunks, 1)
	assert.Equal(t, 1, fileChunks[0].StartLine, "File chunk should start at line 1")
}

func TestJavaScriptChunker_ChunkProperties_Content(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function greet(name) {
    return "Hello, " + name;
}`

	file := createJavaScriptSourceFile("/test/content.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	greetFunc := findChunkByName(result.Chunks, "greet")
	require.NotNil(t, greetFunc)

	// Content should not be empty
	assert.NotEmpty(t, greetFunc.Content, "Chunk should have content")
	assert.Contains(t, greetFunc.Content, "greet", "Content should contain function name")
	assert.Contains(t, greetFunc.Content, "Hello", "Content should contain function body")
}

func TestJavaScriptChunker_ChunkProperties_FilePath(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function test() { }`
	expectedPath := "/test/specific/path.js"

	file := createJavaScriptSourceFile(expectedPath, source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// All chunks should have the correct file path
	for _, chunk := range result.Chunks {
		assert.Equal(t, expectedPath, chunk.FilePath, "All chunks should have correct file path")
	}
}

func TestJavaScriptChunker_ChunkProperties_Language(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `interface Test { id: number; }`

	file := createJavaScriptSourceFile("/test/lang.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// All chunks should have the correct language
	for _, chunk := range result.Chunks {
		assert.Equal(t, "typescript", chunk.Language, "All chunks should have correct language")
	}
}

func TestJavaScriptChunker_ChunkProperties_Hashes(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function test() { return 42; }`

	file := createJavaScriptSourceFile("/test/hashes.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// All chunks should have ID and content hash
	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ID, "Chunk should have an ID")
		assert.NotEmpty(t, chunk.ContentHash, "Chunk should have a content hash")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestJavaScriptChunker_EmptyFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := ``

	file := createJavaScriptSourceFile("/test/empty.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should still have a file chunk even for empty files
	fileChunks := findChunksByLevel(result.Chunks, models.ChunkLevelFile)
	assert.Len(t, fileChunks, 1, "Should have one file chunk even for empty file")
}

func TestJavaScriptChunker_OnlyComments(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `// This is a comment
/* Multi-line
   comment */`

	file := createJavaScriptSourceFile("/test/comments.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should only have file chunk when there's no code
	assert.GreaterOrEqual(t, len(result.Chunks), 1, "Should have at least file chunk")
}

func TestJavaScriptChunker_NestedClasses(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Outer {
    static Inner = class {
        method() {
            return "inner";
        }
    };

    outerMethod() {
        return "outer";
    }
}`

	file := createJavaScriptSourceFile("/test/nested.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	outerClass := findChunkByName(result.Chunks, "Outer")
	assert.NotNil(t, outerClass, "Should find 'Outer' class")

	outerMethod := findChunkByName(result.Chunks, "outerMethod")
	assert.NotNil(t, outerMethod, "Should find 'outerMethod'")
}

func TestJavaScriptChunker_MixedContent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }
}

function greet(name) {
    return "Hello, " + name;
}

const farewell = (name) => {
    return "Goodbye, " + name;
};

export default Calculator;`

	file := createJavaScriptSourceFile("/test/mixed.js", source, "javascript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have file + class + method + 2 top-level functions
	assert.GreaterOrEqual(t, len(result.Chunks), 5, "Should have multiple chunks")

	calcClass := findChunkByName(result.Chunks, "Calculator")
	assert.NotNil(t, calcClass, "Should find Calculator class")

	addMethod := findChunkByName(result.Chunks, "add")
	assert.NotNil(t, addMethod, "Should find add method")

	greetFunc := findChunkByName(result.Chunks, "greet")
	assert.NotNil(t, greetFunc, "Should find greet function")

	farewellFunc := findChunkByName(result.Chunks, "farewell")
	assert.NotNil(t, farewellFunc, "Should find farewell arrow function")
}

func TestJavaScriptChunker_CancelledContext(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	source := `class Test { }`
	file := createJavaScriptSourceFile("/test/cancelled.js", source, "javascript")

	// Should return error when context is cancelled
	_, err = chunker.Chunk(ctx, file)
	assert.Error(t, err, "Should return error when context is cancelled")
}

// =============================================================================
// Signature Tests
// =============================================================================

func TestJavaScriptChunker_MethodSignature(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `class UserService {
    async getUser(id: number): Promise<User> {
        return { id, name: "test" };
    }
}`

	file := createJavaScriptSourceFile("/test/signature.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	getUser := findChunkByName(result.Chunks, "getUser")
	require.NotNil(t, getUser)

	// Signature should capture the function declaration
	assert.NotEmpty(t, getUser.Signature, "Method should have a signature")
}

func TestJavaScriptChunker_InterfaceSignature(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `export interface UserRepository {
    findById(id: string): User | null;
}`

	file := createJavaScriptSourceFile("/test/interface_sig.ts", source, "typescript")

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	userRepo := findChunkByName(result.Chunks, "UserRepository")
	require.NotNil(t, userRepo)

	// Signature should capture the interface declaration
	assert.NotEmpty(t, userRepo.Signature, "Interface should have a signature")
}
