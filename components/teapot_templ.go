// Code generated by templ - DO NOT EDIT.

// templ: version: v0.2.663
package components

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import "context"
import "io"
import "bytes"

func TeapotPage() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, templ_7745c5c3_W io.Writer) (templ_7745c5c3_Err error) {
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templ_7745c5c3_W.(*bytes.Buffer)
		if !templ_7745c5c3_IsBuffer {
			templ_7745c5c3_Buffer = templ.GetBuffer()
			defer templ.ReleaseBuffer(templ_7745c5c3_Buffer)
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = WrapContents(Teapot()).Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if !templ_7745c5c3_IsBuffer {
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteTo(templ_7745c5c3_W)
		}
		return templ_7745c5c3_Err
	})
}

func logTeapotRequest() templ.ComponentScript {
	return templ.ComponentScript{
		Name: `__templ_logTeapotRequest_fcfc`,
		Function: `function __templ_logTeapotRequest_fcfc(){fetch('https://api.neuralnexus.dev/api/v1/teapot')
		.then((response) => {
			if (response.status !== 418) {
				console.error('Failed to log teapot request:', response.statusText);
			}
		})
		.catch((error) => {
			console.error('Failed to log teapot request:', error);
		});
}`,
		Call:       templ.SafeScript(`__templ_logTeapotRequest_fcfc`),
		CallInline: templ.SafeScriptInline(`__templ_logTeapotRequest_fcfc`),
	}
}

func Teapot() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, templ_7745c5c3_W io.Writer) (templ_7745c5c3_Err error) {
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templ_7745c5c3_W.(*bytes.Buffer)
		if !templ_7745c5c3_IsBuffer {
			templ_7745c5c3_Buffer = templ.GetBuffer()
			defer templ.ReleaseBuffer(templ_7745c5c3_Buffer)
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var2 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var2 == nil {
			templ_7745c5c3_Var2 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = logTeapotRequest().Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<meta property=\"og:site_name\" content=\"Powered by NeuralNexus.dev\"><meta name=\"theme-color\" data-react-helmet=\"true\" content=\"#7C0014\"><meta property=\"og:title\" content=\"418 I&#39;m a teapot\"><meta property=\"og:description\" content=\"You requested a cup of coffee, but I&#39;m a teapot.\"><meta property=\"og:url\" content=\"https://neuralnexus.dev/teapot\"><meta property=\"og:image\" content=\"https://neuralnexus.dev/public/teapot.jpg\"><div class=\"flex flex-col items-center justify-center gap-4 text-center\"><img src=\"/public/teapot.jpg\" width=\"200\" height=\"200\" alt=\"Teapot\" class=\"rounded-full border-4 border-white\" style=\"aspect-ratio: 200 / 200; object-fit: cover;\"><div class=\"space-y-2\"><h1 class=\"text-4xl font-bold\">418 I'm a teapot</h1><p class=\"text-sm text-gray-500\">You requested a cup of coffee, but I'm a teapot.</p></div><a class=\"rounded-full w-full max-w-xs\" href=\"/\">Brew another cup</a></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if !templ_7745c5c3_IsBuffer {
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteTo(templ_7745c5c3_W)
		}
		return templ_7745c5c3_Err
	})
}
