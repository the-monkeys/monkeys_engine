package utils

import (
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
)

var Address string

func init() {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf("cannot load the config: %v", err)
	}
	Address = cfg.Authentication.EmailVerificationAddr
	logrus.Infof("✅ the email verification address is: %v", Address)
}

func ResetPasswordTemplate(firstName, LastName, secret string, username string) string {
	return `<!DOCTYPE html PUBLIC>
	<html xmlns="http://www.w3.org/1999/xhtml">
	  <head>
		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
		<meta name="x-apple-disable-message-reformatting" />
		<meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
		<meta name="color-scheme" content="light dark" />
		<meta name="supported-color-schemes" content="light dark" />
		<title></title>
		<style type="text/css" rel="stylesheet" media="all">
		/* Base ------------------------------ */
		
		@import url("https://fonts.googleapis.com/css?family=Nunito+Sans:400,700&display=swap");
		body {
		  width: 100% !important;
		  height: 100%;
		  margin: 0;
		  -webkit-text-size-adjust: none;
		}
		
		a {
		  color: #3869D4;
		}
		
		a img {
		  border: none;
		}
		
		td {
		  word-break: break-word;
		}
		
		.preheader {
		  display: none !important;
		  visibility: hidden;
		  mso-hide: all;
		  font-size: 1px;
		  line-height: 1px;
		  max-height: 0;
		  max-width: 0;
		  opacity: 0;
		  overflow: hidden;
		}
		/* Type ------------------------------ */
		
		body,
		td,
		th {
		  font-family: "Nunito Sans", Helvetica, Arial, sans-serif;
		}
		
		h1 {
		  margin-top: 0;
		  color: #333333;
		  font-size: 22px;
		  font-weight: bold;
		  text-align: left;
		}
		
		h2 {
		  margin-top: 0;
		  color: #333333;
		  font-size: 16px;
		  font-weight: bold;
		  text-align: left;
		}
		
		h3 {
		  margin-top: 0;
		  color: #333333;
		  font-size: 14px;
		  font-weight: bold;
		  text-align: left;
		}
		
		td,
		th {
		  font-size: 16px;
		}
		
		p,
		ul,
		ol,
		blockquote {
		  margin: .4em 0 1.1875em;
		  font-size: 16px;
		  line-height: 1.625;
		}
		
		p.sub {
		  font-size: 13px;
		}
		/* Utilities ------------------------------ */
		
		.align-right {
		  text-align: right;
		}
		
		.align-left {
		  text-align: left;
		}
		
		.align-center {
		  text-align: center;
		}
		
		.u-margin-bottom-none {
		  margin-bottom: 0;
		}
		/* Buttons ------------------------------ */
		
		.button {
		  background-color: #3869D4;
		  border-top: 10px solid #3869D4;
		  border-right: 18px solid #3869D4;
		  border-bottom: 10px solid #3869D4;
		  border-left: 18px solid #3869D4;
		  display: inline-block;
		  color: #FFF;
		  text-decoration: none;
		  border-radius: 3px;
		  box-shadow: 0 2px 3px rgba(0, 0, 0, 0.16);
		  -webkit-text-size-adjust: none;
		  box-sizing: border-box;
		}
		
		.button--green {
		  background-color: #22BC66;
		  border-top: 10px solid #22BC66;
		  border-right: 18px solid #22BC66;
		  border-bottom: 10px solid #22BC66;
		  border-left: 18px solid #22BC66;
		}
		
		.button--red {
		  background-color: #FF6136;
		  border-top: 10px solid #FF6136;
		  border-right: 18px solid #FF6136;
		  border-bottom: 10px solid #FF6136;
		  border-left: 18px solid #FF6136;
		}
		
		@media only screen and (max-width: 500px) {
		  .button {
			width: 100% !important;
			text-align: center !important;
		  }
		}
		/* Attribute list ------------------------------ */
		
		.attributes {
		  margin: 0 0 21px;
		}
		
		.attributes_content {
		  background-color: #F4F4F7;
		  padding: 16px;
		}
		
		.attributes_item {
		  padding: 0;
		}
		/* Related Items ------------------------------ */
		
		.related {
		  width: 100%;
		  margin: 0;
		  padding: 25px 0 0 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		}
		
		.related_item {
		  padding: 10px 0;
		  color: #CBCCCF;
		  font-size: 15px;
		  line-height: 18px;
		}
		
		.related_item-title {
		  display: block;
		  margin: .5em 0 0;
		}
		
		.related_item-thumb {
		  display: block;
		  padding-bottom: 10px;
		}
		
		.related_heading {
		  border-top: 1px solid #CBCCCF;
		  text-align: center;
		  padding: 25px 0 10px;
		}
		/* Discount Code ------------------------------ */
		
		.discount {
		  width: 100%;
		  margin: 0;
		  padding: 24px;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		  background-color: #F4F4F7;
		  border: 2px dashed #CBCCCF;
		}
		
		.discount_heading {
		  text-align: center;
		}
		
		.discount_body {
		  text-align: center;
		  font-size: 15px;
		}
		/* Social Icons ------------------------------ */
		
		.social {
		  width: auto;
		}
		
		.social td {
		  padding: 0;
		  width: auto;
		}
		
		.social_icon {
		  height: 20px;
		  margin: 0 8px 10px 8px;
		  padding: 0;
		}
		/* Data table ------------------------------ */
		
		.purchase {
		  width: 100%;
		  margin: 0;
		  padding: 35px 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		}
		
		.purchase_content {
		  width: 100%;
		  margin: 0;
		  padding: 25px 0 0 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		}
		
		.purchase_item {
		  padding: 10px 0;
		  color: #51545E;
		  font-size: 15px;
		  line-height: 18px;
		}
		
		.purchase_heading {
		  padding-bottom: 8px;
		  border-bottom: 1px solid #EAEAEC;
		}
		
		.purchase_heading p {
		  margin: 0;
		  color: #85878E;
		  font-size: 12px;
		}
		
		.purchase_footer {
		  padding-top: 15px;
		  border-top: 1px solid #EAEAEC;
		}
		
		.purchase_total {
		  margin: 0;
		  text-align: right;
		  font-weight: bold;
		  color: #333333;
		}
		
		.purchase_total--label {
		  padding: 0 15px 0 0;
		}
		
		body {
		  background-color: #F2F4F6;
		  color: #51545E;
		}
		
		p {
		  color: #51545E;
		}
		
		.email-wrapper {
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		  background-color: #F2F4F6;
		}
		
		.email-content {
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		}
		/* Masthead ----------------------- */
		
		.email-masthead {
		  padding: 25px 0;
		  text-align: center;
		}
		
		.email-masthead_logo {
		  width: 94px;
		}
		
		.email-masthead_name {
		  font-size: 16px;
		  font-weight: bold;
		  color: #A8AAAF;
		  text-decoration: none;
		  text-shadow: 0 1px 0 white;
		}
		/* Body ------------------------------ */
		
		.email-body {
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		}
		
		.email-body_inner {
		  width: 570px;
		  margin: 0 auto;
		  padding: 0;
		  -premailer-width: 570px;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		  background-color: #FFFFFF;
		}
		
		.email-footer {
		  width: 570px;
		  margin: 0 auto;
		  padding: 0;
		  -premailer-width: 570px;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		  text-align: center;
		}
		
		.email-footer p {
		  color: #A8AAAF;
		}
		
		.body-action {
		  width: 100%;
		  margin: 30px auto;
		  padding: 0;
		  -premailer-width: 100%;
		  -premailer-cellpadding: 0;
		  -premailer-cellspacing: 0;
		  text-align: center;
		}
		
		.body-sub {
		  margin-top: 25px;
		  padding-top: 25px;
		  border-top: 1px solid #EAEAEC;
		}
		
		.content-cell {
		  padding: 45px;
		}
		/*Media Queries ------------------------------ */
		
		@media only screen and (max-width: 600px) {
		  .email-body_inner,
		  .email-footer {
			width: 100% !important;
		  }
		}
		
		@media (prefers-color-scheme: dark) {
		  body,
		  .email-body,
		  .email-body_inner,
		  .email-content,
		  .email-wrapper,
		  .email-masthead,
		  .email-footer {
			background-color: #333333 !important;
			color: #FFF !important;
		  }
		  p,
		  ul,
		  ol,
		  blockquote,
		  h1,
		  h2,
		  h3,
		  span,
		  .purchase_item {
			color: #FFF !important;
		  }
		  .attributes_content,
		  .discount {
			background-color: #222 !important;
		  }
		  .email-masthead_name {
			text-shadow: none !important;
		  }
		}
		
		:root {
		  color-scheme: light dark;
		  supported-color-schemes: light dark;
		}
		</style>
		<!--[if mso]>
		<style type="text/css">
		  .f-fallback  {
			font-family: Arial, sans-serif;
		  }
		</style>
	  <![endif]-->
	  </head>
	  <body>
		<span class="preheader">Use this link to reset your password. The link is only valid for 24 hours.</span>
		<table class="email-wrapper" width="100%" cellpadding="0" cellspacing="0" role="presentation">
		  <tr>
			<td align="center">
			  <table class="email-content" width="100%" cellpadding="0" cellspacing="0" role="presentation">
				<tr>
				  <td class="email-masthead">
					<a href="https://www.themonkeys.life" class="f-fallback email-masthead_name">
					The Monkeys
				  </a>
				  </td>
				</tr>
				<!-- Email Body -->
				<tr>
				  <td class="email-body" width="570" cellpadding="0" cellspacing="0">
					<table class="email-body_inner" align="center" width="570" cellpadding="0" cellspacing="0" role="presentation">
					  <!-- Body content -->
					  <tr>
						<td class="content-cell">
						  <div class="f-fallback">
							<h1>Hi ` + firstName + ` ` + LastName + `,</h1>
							<p>You recently requested to reset your password for your The Monkeys account. Use the button below to reset it. <strong>This password reset link is only valid for the next 5 minutes.</strong></p>
							<!-- Action -->
							<table class="body-action" align="center" width="100%" cellpadding="0" cellspacing="0" role="presentation">
							  <tr>
								<td align="center">
								
								  <table width="100%" border="0" cellspacing="0" cellpadding="0" role="presentation">
									<tr>
									  <td align="center">
										<a href="` + Address + `/auth/reset-password?user=` + username + `&evpw=` + secret + `" class="f-fallback button button--green" target="_blank">Reset your password</a>
									  </td>
									</tr>
								  </table>
								</td>
							  </tr>
							</table>
							<p>If you did not request a password reset, please ignore this email or <a href="{{https://www.themonkeys.life/contact}}">contact support</a> if you have questions.</p>
							<p>Thanks,
							  <br>The Monkeys team</p>
							<!-- Sub copy -->
							
						  </div>
						</td>
					  </tr>
					</table>
				  </td>
				</tr>
			  </table>
			</td>
		  </tr>
		</table>
	  </body>
	</html>`

}

func EmailVerificationHTML(firstName, lastName, username, secret string) string {
	return `<!DOCTYPE html>
<html lang="en">

<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/4.7.0/css/font-awesome.min.css">
  <title>Welcome to Monkeys</title>
  <style>
    body {
      font-family: Arial, sans-serif;
      background-color: #f3f3f3;
      margin: 0;
      padding: 20px;
      display: flex;
      justify-content: center;
      align-items: center;
      height: 100vh;
    }

    h1 {
      color: #333;
      margin-bottom: 20px;
      text-align: center;
    }

    .logo {
      color: #ff5252;
      display: inline-block;
    }

    p {
      color: #000000;
      line-height: 1.2;
      margin-bottom: 20px;
    }

    .btn {
      background-color: #ff5252;
      color: white;
      border: none;
      padding: 10px 20px;
      text-transform: uppercase;
      border-radius: 5px;
      cursor: pointer;
    }

    .footer {
      display: flex;
      justify-content: center;
      align-items: center;
      gap: 20px;
      margin-top: 40px;
    }

    a {
      /* Add your styles here */
      text-decoration: none;
      color: #ff5252;
      display: inline-block;
    }
  </style>
</head>

<body>
  <div class="container">
    <h1>Welcome to <div class="logo">Monkeys</div>
    </h1>

    <p>Hello ` + firstName + ` ` + lastName + `,</p>

    <p>Thanks for joining The Monkeys! We're excited to have you on board.</p>

    <p>To complete your registration and start exploring everything The Monkeys has to offer, please verify your email address. This helps ensure your account security and keeps you informed about important updates.</p>

    <p>It's easy! Just click the button below to confirm your email:</p>

	<a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank"><button class="btn">Verify Email Address</button></a>
    
	<p><b>Alternatively</b>, you can copy and paste this link into your web browser:</p>
    <a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank">` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `</a>

    <p>This link will expire in 24 hours for your security. If you don't verify your email within that time frame, you can request a new link anytime.</p>
    <p>Once you verify your email, you'll be ready to dive in and start using The Monkeys!</p>
    <p>If you have any trouble verifying your email, please feel free to contact The Monkeys support team at [support email address]. We're happy to help!</p>
    <p>Welcome to the community,</p>

    <p>Thanks,</p>
    <p>The Monkeys Team</p>

    <div class="footer">
      <a href="https://github.com/the-monkeys" target="_blank"><i style="font-size:24px" class="fa">&#xf09b;</i></a>
      <a href="https://www.instagram.com/_the__monkeys_" target="_blank"><i style="font-size:24px" class="fa">&#xf16d;</i></a>
      <a href="https://x.com/TheMonkeysLife" target="_blank"><i style="font-size:24px" class="fa">&#xf099;</i></a>
    </div>

  </div>
</body>

</html>`
}

// func EmailVerificationHTMLa(username, secret string) string {
// 	return `<!DOCTYPE html>
// 	<html>
// 	<head>

// 	  <meta charset="utf-8">
// 	  <meta http-equiv="x-ua-compatible" content="ie=edge">
// 	  <title>The Monkeys</title>
// 	  <meta name="viewport" content="width=device-width, initial-scale=1">
// 	  <style type="text/css">

// 	  </style>

// 	</head>
// 	<body style="background-color: #e9ecef;">

// 		<div style="display: none; font-size: 1px; color: #fefefe; line-height: 1px; font-family: 'Lato', Helvetica, Arial, sans-serif; max-height: 0px; max-width: 0px; opacity: 0; overflow: hidden;"> We're thrilled to have you here! Get ready to dive into your new account. </div>
// 		<table border="0" cellpadding="0" cellspacing="0" width="100%">
// 			<!-- LOGO -->
// 			<tr>
// 				<td bgcolor="#FFA73B" align="center">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td align="center" valign="top" style="padding: 40px 10px 40px 10px;"> </td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#FFA73B" align="center" style="padding: 0px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#ffffff" align="center" valign="top" style="padding: 40px 20px 20px 20px; border-radius: 4px 4px 0px 0px; color: #111111; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 48px; font-weight: 400; letter-spacing: 4px; line-height: 48px;">
// 								<h1 style="font-size: 48px; font-weight: 400; margin: 2;">The Monkeys</h1> <img src=" https://img.icons8.com/clouds/100/000000/handshake.png" width="125" height="120" style="display: block; border: 0px;" />
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#f4f4f4" align="center" style="padding: 0px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 20px 30px 40px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">We're excited to have you get started. First, you need to confirm your account. Just press the button below.</p>
// 							</td>
// 						</tr>
// 						<tr>
// 							<td bgcolor="#ffffff" align="left">
// 								<table width="100%" border="0" cellspacing="0" cellpadding="0">
// 									<tr>
// 										<td bgcolor="#ffffff" align="center" style="padding: 20px 30px 60px 30px;">
// 											<table border="0" cellspacing="0" cellpadding="0">
// 												<tr>
// 													<td align="center" style="border-radius: 3px;" bgcolor="#FFA73B"><a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank" style="font-size: 20px; font-family: Helvetica, Arial, sans-serif; color: #ffffff; text-decoration: none; color: #ffffff; text-decoration: none; padding: 15px 25px; border-radius: 2px; border: 1px solid #FFA73B; display: inline-block;">Confirm Account</a></td>
// 												</tr>
// 											</table>
// 										</td>
// 									</tr>
// 								</table>
// 							</td>
// 						</tr> <!-- COPY -->
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 0px 30px 0px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">If that doesn't work, copy and paste the following link in your browser:</p>
// 							</td>
// 						</tr> <!-- COPY -->
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 20px 30px 20px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;"><a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank" style="color: #FFA73B;">` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `</a></p>
// 							</td>
// 						</tr>
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 0px 30px 20px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">If you have any questions, just reply to this email—we're always happy to help out.</p>
// 							</td>
// 						</tr>
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 0px 30px 40px 30px; border-radius: 0px 0px 4px 4px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">Cheers,<br>The Monkeys Team</p>
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#f4f4f4" align="center" style="padding: 30px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#FFECD1" align="center" style="padding: 30px 30px 30px 30px; border-radius: 4px 4px 4px 4px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<h2 style="font-size: 20px; font-weight: 400; color: #111111; margin: 0;">Need more help?</h2>
// 								<p style="margin: 0;"><a href="themonkeys.life" target="_blank" style="color: #FFA73B;">We&rsquo;re here to help you out</a></p>
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#f4f4f4" align="center" style="padding: 0px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#f4f4f4" align="left" style="padding: 0px 30px 30px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 14px; font-weight: 400; line-height: 18px;"> <br>
// 								<p style="margin: 0;">If these emails get annoying, please feel free to <a href="#" target="_blank" style="color: #111111; font-weight: 700;">unsubscribe</a>.</p>
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 		</table>

// 	</body>
// 	</html>`
// }
