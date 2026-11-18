package openapi

// OAuth2RedirectScriptCSPSource is the Content-Security-Policy source expression for the one inline script of
// the OAuth2 redirect page that Config.OAuth2RedirectPath serves.
const OAuth2RedirectScriptCSPSource = "'sha256-pcijTgnklzc4LMUBlEbux8PbNGGWP8W/5Eg2HTxVSek='"

// oauth2RedirectScript is oauth2-redirect.js of swagger-ui-dist 5.32.6, unchanged. Swagger UI opens the
// page that carries it in a popup to receive the authorization code or token from an OAuth2 provider.
const oauth2RedirectScript = `"use strict";function run(){var e,r,t,a=window.opener.swaggerUIRedirectOauth2,o=a.state,n=a.redirectUrl;if((t=(r=/code|token|error/.test(window.location.hash)?window.location.hash.substring(1).replace("?","&"):location.search.substring(1)).split("&")).forEach((function(e,r,t){t[r]='"'+e.replace("=",'":"')+'"'})),e=(r=r?JSON.parse("{"+t.join()+"}",(function(e,r){return""===e?r:decodeURIComponent(r)})):{}).state===o,"accessCode"!==a.auth.schema.get("flow")&&"authorizationCode"!==a.auth.schema.get("flow")&&"authorization_code"!==a.auth.schema.get("flow")||a.auth.code)a.callback({auth:a.auth,token:r,isValid:e,redirectUrl:n});else if(e||a.errCb({authId:a.auth.name,source:"auth",level:"warning",message:"Authorization may be unsafe, passed state was changed in server Passed state wasn't returned from auth server"}),r.code)delete a.state,a.auth.code=r.code,a.callback({auth:a.auth,redirectUrl:n});else{let e;r.error&&(e="["+r.error+"]: "+(r.error_description?r.error_description+". ":"no accessCode received from the server. ")+(r.error_uri?"More info: "+r.error_uri:"")),a.errCb({authId:a.auth.name,source:"auth",level:"error",message:e||"[Authorization failed]: no accessCode received from the server"})}window.close()}"loading"!==document.readyState?run():document.addEventListener("DOMContentLoaded",(function(){run()}));`

// oauth2RedirectPage is the redirect page of swagger-ui-dist with its script inlined, so a strict
// Content-Security-Policy can allow it by hash (see OAuth2RedirectScriptCSPSource).
const oauth2RedirectPage = `<!doctype html>
<html lang="en-US">
<body>
<script>` + oauth2RedirectScript + `</script>
</body>
</html>
`
