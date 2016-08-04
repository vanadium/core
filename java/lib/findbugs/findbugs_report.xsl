<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
<xsl:output method="text" omit-xml-declaration="yes" indent="no"/>
<xsl:template match="/">
	<xsl:for-each select="BugCollection/BugInstance">
		<xsl:text>In </xsl:text><xsl:value-of select="SourceLine/@sourcepath"/><xsl:text>:&#xd;&#xa;</xsl:text>
		<xsl:value-of select="LongMessage"/>
		<xsl:text>&#xd;&#xa;</xsl:text>
		<xsl:for-each select="SourceLine/Message"><xsl:text>&#x9;</xsl:text><xsl:value-of select="."/><xsl:text>&#xd;&#xa;</xsl:text></xsl:for-each>
		<xsl:text>&#xd;&#xa;</xsl:text>
	</xsl:for-each>
</xsl:template>
</xsl:stylesheet>
